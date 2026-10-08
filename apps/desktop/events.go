package main

import "sync"

// Event is one live update for the frontend: a Wails event in the window, an
// SSE message on GET /rpc/events in `serve`.
type Event struct {
	Name string
	Data any
}

// Events fans live updates out to the Wails window (window, when set) and to
// every open /rpc/events stream.
type Events struct {
	mu     sync.Mutex
	window func(name string, data any)
	subs   map[chan Event]struct{}
}

// NewEvents returns an Events with no window and no streams.
func NewEvents() *Events {
	return &Events{subs: map[chan Event]struct{}{}}
}

// Emit sends an event to the window and every stream. A stream that is not
// keeping up misses it rather than blocking the sender.
func (e *Events) Emit(name string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.window != nil {
		e.window(name, data)
	}
	for ch := range e.subs {
		select {
		case ch <- Event{Name: name, Data: data}:
		default:
		}
	}
}

// Subscribe opens a stream; cancel closes it.
func (e *Events) Subscribe() (events <-chan Event, cancel func()) {
	ch := make(chan Event, 16)
	e.mu.Lock()
	e.subs[ch] = struct{}{}
	e.mu.Unlock()
	return ch, func() {
		e.mu.Lock()
		delete(e.subs, ch)
		e.mu.Unlock()
	}
}

// SetWindow routes events into the Wails window.
func (e *Events) SetWindow(emit func(name string, data any)) {
	e.mu.Lock()
	e.window = emit
	e.mu.Unlock()
}
