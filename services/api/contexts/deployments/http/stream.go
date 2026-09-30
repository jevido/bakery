package http

import (
	"context"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/app/sse"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

const (
	pollEvery = 500 * time.Millisecond
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
	stream, ok := sse.Start(ctx)
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
			stream.Event("line", strconv.FormatUint(l.ID, 10), map[string]any{"id": l.ID, "stream": l.Stream, "line": l.Line})
		}
		if len(lines) == batch {
			continue // more waiting; send before checking the status
		}
		if d, err = c.service.Deployment(reqCtx, id); err != nil {
			return nil
		}
		if d.Status != lastStatus {
			lastStatus = d.Status
			stream.Event("status", "", ToJSON(d))
		}
		if !d.Status.Active() {
			// Lines written just before the final status was saved.
			if more, err := c.service.LogAfter(reqCtx, id, after, batch); err == nil && len(more) > 0 {
				continue
			}
			stream.Event("end", "", map[string]any{"status": d.Status})
			return nil
		}
		if time.Since(lastPing) > sse.PingEvery {
			stream.Raw(": ping\n\n")
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
	stream, ok := sse.Start(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}

	sse.Follow(reqCtx, stream, c.shutdown, func(ctx context.Context, out func(stream, line string)) (bool, error) {
		return c.containers(ctx, id, 200, out)
	})
	return nil
}
