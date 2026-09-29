package http

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strconv"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

const (
	pollEvery = 500 * time.Millisecond
	pingEvery = 20 * time.Second
	batch     = 500
)

// ContainerLogs follows the running Container of an Application, handing
// each line to out, until ctx ends or the Container stops. found is false
// when the Application has no running Container.
type ContainerLogs func(ctx context.Context, applicationID uint64, tail int, out func(stream, line string)) (found bool, err error)

// StreamController serves the live logs as server-sent events. Streams also
// end when shutdown is done, so a stopping API does not wait on open tabs.
type StreamController struct {
	service    *app.Service
	containers ContainerLogs
	isNotFound func(error) bool
	shutdown   <-chan struct{}
}

func NewStreamController(service *app.Service, containers ContainerLogs, isNotFound func(error) bool, shutdown <-chan struct{}) *StreamController {
	return &StreamController{service: service, containers: containers, isNotFound: isNotFound, shutdown: shutdown}
}

// sse writes events; it is safe for the log callback and the ping ticker to
// use at once.
type sse struct {
	mu sync.Mutex
	w  nethttp.ResponseWriter
	f  nethttp.Flusher
}

func startSSE(ctx contractshttp.Context) (*sse, bool) {
	w := ctx.Response().Writer()
	f, ok := w.(nethttp.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(contractshttp.StatusOK)
	s := &sse{w: w, f: f}
	s.raw("retry: 3000\n\n")
	return s, true
}

func (s *sse) raw(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprint(s.w, text)
	s.f.Flush()
}

// event sends one event; id (when not empty) is what EventSource sends back
// as Last-Event-ID on reconnect.
func (s *sse) event(name, id string, data any) {
	b, _ := json.Marshal(data)
	text := ""
	if id != "" {
		text = "id: " + id + "\n"
	}
	s.raw(text + "event: " + name + "\ndata: " + string(b) + "\n\n")
}

// DeploymentLog sends the stored log lines, then new ones as they are
// written, and `status` events when the status changes. After a final
// status it sends `end`; the client closes then, or EventSource would
// reconnect. A reconnect resumes after Last-Event-ID.
func (c *StreamController) DeploymentLog(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	reqCtx := ctx.Request().Origin().Context()
	d, err := c.service.Deployment(reqCtx, id)
	if err != nil {
		if c.isNotFound(err) {
			return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
		}
		return respond.ServerError(ctx, err)
	}
	after, _ := strconv.ParseUint(ctx.Request().Header("Last-Event-ID"), 10, 64)
	stream, ok := startSSE(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}

	lastStatus := domain.Status("")
	lastPing := time.Now()
	for {
		lines, err := c.service.LogAfter(reqCtx, id, after, batch)
		if err != nil {
			return nil
		}
		for _, l := range lines {
			after = l.ID
			stream.event("line", strconv.FormatUint(l.ID, 10), map[string]any{"id": l.ID, "stream": l.Stream, "line": l.Line})
		}
		if len(lines) == batch {
			continue // more waiting; send before checking the status
		}
		if d, err = c.service.Deployment(reqCtx, id); err != nil {
			return nil
		}
		if d.Status != lastStatus {
			lastStatus = d.Status
			stream.event("status", "", ToJSON(d))
		}
		if !d.Status.Active() {
			// Lines written just before the final status was saved.
			if more, err := c.service.LogAfter(reqCtx, id, after, batch); err == nil && len(more) > 0 {
				continue
			}
			stream.event("end", "", map[string]any{"status": d.Status})
			return nil
		}
		if time.Since(lastPing) > pingEvery {
			stream.raw(": ping\n\n")
			lastPing = time.Now()
		}
		select {
		case <-reqCtx.Done():
			return nil
		case <-c.shutdown:
			return nil
		case <-time.After(pollEvery):
		}
	}
}

// ContainerLogs follows the Application's running Container: the last 200
// lines, then new ones. It sends `end` when there is no running Container or
// it stops (a redeploy replaced it); the client may reconnect then.
func (c *StreamController) ContainerLogs(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	reqCtx := ctx.Request().Origin().Context()
	if _, err := c.service.Deployments(reqCtx, id); err != nil { // checks the Application exists
		if c.isNotFound(err) {
			return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
		}
		return respond.ServerError(ctx, err)
	}
	stream, ok := startSSE(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}

	followCtx, cancel := context.WithCancel(reqCtx)
	defer cancel()
	go func() {
		ping := time.NewTicker(pingEvery)
		defer ping.Stop()
		for {
			select {
			case <-followCtx.Done():
				return
			case <-c.shutdown:
				cancel()
				return
			case <-ping.C:
				stream.raw(": ping\n\n")
			}
		}
	}()
	found, err := c.containers(followCtx, id, 200, func(s, line string) {
		stream.event("line", "", map[string]any{"stream": s, "line": line})
	})
	if followCtx.Err() != nil {
		return nil
	}
	reason := "stopped"
	switch {
	case err != nil:
		reason = "error"
	case !found:
		reason = "not running"
	}
	stream.event("end", "", map[string]any{"reason": reason})
	return nil
}
