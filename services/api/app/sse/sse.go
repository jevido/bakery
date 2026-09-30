// Package sse writes server-sent events, for the live log streams of more
// than one context.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
)

// PingEvery is how often an idle stream sends a comment, so proxies keep it
// open.
const PingEvery = 20 * time.Second

// Stream writes events; it is safe for a log callback and a ping ticker to
// use at once.
type Stream struct {
	mu sync.Mutex
	w  nethttp.ResponseWriter
	f  nethttp.Flusher
}

// Start answers 200 with the event-stream headers; ok is false when the
// response cannot be flushed.
func Start(ctx contractshttp.Context) (*Stream, bool) {
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
	s := &Stream{w: w, f: f}
	s.Raw("retry: 3000\n\n")
	return s, true
}

// Raw writes text as is and flushes it.
func (s *Stream) Raw(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprint(s.w, text)
	s.f.Flush()
}

// Event sends one event; id (when not empty) is what EventSource sends back
// as Last-Event-ID on reconnect.
func (s *Stream) Event(name, id string, data any) {
	b, _ := json.Marshal(data)
	text := ""
	if id != "" {
		text = "id: " + id + "\n"
	}
	s.Raw(text + "event: " + name + "\ndata: " + string(b) + "\n\n")
}

// Follow sends what follow hands out as `line` events ({stream, line}),
// pinging while it is quiet, until follow returns, the client leaves or
// shutdown closes. When follow returns it sends `end` with a reason:
// "stopped", "error", or "not running" when found is false.
func Follow(reqCtx context.Context, s *Stream, shutdown <-chan struct{}, follow func(ctx context.Context, out func(stream, line string)) (found bool, err error)) {
	ctx, cancel := context.WithCancel(reqCtx)
	defer cancel()
	go func() {
		ping := time.NewTicker(PingEvery)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-shutdown:
				cancel()
				return
			case <-ping.C:
				s.Raw(": ping\n\n")
			}
		}
	}()
	found, err := follow(ctx, func(stream, line string) {
		s.Event("line", "", map[string]any{"stream": stream, "line": line})
	})
	if ctx.Err() != nil {
		return
	}
	reason := "stopped"
	switch {
	case err != nil:
		reason = "error"
	case !found:
		reason = "not running"
	}
	s.Event("end", "", map[string]any{"reason": reason})
}
