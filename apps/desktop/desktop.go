package main

// version is the Desktop app's version, set at build time with
// -ldflags "-X main.version=…".
var version = "dev"

// Desktop holds every method the frontend calls. The Wails window binds it
// (wails3 generate bindings turns its exported methods into
// frontend/bindings), and `serve` exposes the same methods on /rpc, so both
// transports run the same code.
type Desktop struct {
	events *Events
}

// NewDesktop returns the Desktop service, sending its live updates to events.
func NewDesktop(events *Events) *Desktop {
	return &Desktop{events: events}
}

// Version answers the Desktop app's version.
func (d *Desktop) Version() string {
	return version
}
