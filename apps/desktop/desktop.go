package main

import (
	"sync"

	"github.com/jevido/bakery/apps/desktop/store"
)

// version is the Desktop app's version, set at build time with
// -ldflags "-X main.version=…".
var version = "dev"

// Desktop holds every method the frontend calls. The Wails window binds it
// (wails3 generate bindings turns its exported methods into
// frontend/bindings), and `serve` exposes the same methods on /rpc, so both
// transports run the same code.
type Desktop struct {
	events *Events
	store  *store.Store
	// openURL opens a link in the system browser; nil in `serve`, where the
	// page shows the link instead.
	openURL func(url string) error

	mu          sync.Mutex
	connects    map[uint64]*connecting
	lastConnect uint64
	shown       shown
	cache       map[string]cached
	refreshing  sync.Once
}

// NewDesktop returns the Desktop service, keeping the connected Bakeries in
// bakeries and sending its live updates to events.
func NewDesktop(events *Events, bakeries *store.Store, openURL func(string) error) *Desktop {
	return &Desktop{events: events, store: bakeries, openURL: openURL, connects: map[uint64]*connecting{}, cache: map[string]cached{}}
}

// Version answers the Desktop app's version.
func (d *Desktop) Version() string {
	return version
}
