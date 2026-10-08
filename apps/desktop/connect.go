package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
	"github.com/jevido/bakery/apps/desktop/store"
)

// ConnectedBakery is one connected Bakery as the frontend sees it: never
// its key.
type ConnectedBakery struct {
	Address     string       `json:"address"`
	Member      store.Member `json:"member"`
	ConnectedAt time.Time    `json:"connected_at"`
	Active      bool         `json:"active"`
	SignedOut   bool         `json:"signed_out"`
}

// ConnectStart is what Connect answers: the link to approve this Desktop
// in the browser.
type ConnectStart struct {
	ID          uint64    `json:"id"`
	Address     string    `json:"address"`
	ApprovalURL string    `json:"approval_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Where a connect stands. The first four are the Desktop sign-in's own;
// failed is this app's, when the approved key could not be stored.
const (
	connectPending   = "pending"
	connectApproved  = "approved"
	connectExpired   = "expired"
	connectCancelled = "cancelled"
	connectFailed    = "failed"
)

// ConnectState is a connect's status; Error explains failed.
type ConnectState struct {
	ID      uint64 `json:"id"`
	Address string `json:"address"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// connecting is a connect this Desktop started and is polling. Its id is
// this app's own, not the sign-in's: two Bakeries can hand out the same one.
type connecting struct {
	state  ConnectState
	client *bakery.Client
	signIn bakery.SignIn
	stop   context.CancelFunc
}

// Bakeries lists the connected Bakeries, the active one marked.
func (d *Desktop) Bakeries() ([]ConnectedBakery, error) {
	f, err := d.store.Load()
	if err != nil {
		return nil, err
	}
	out := make([]ConnectedBakery, len(f.Bakeries))
	for i, b := range f.Bakeries {
		out[i] = ConnectedBakery{Address: b.Address, Member: b.Member, ConnectedAt: b.ConnectedAt, Active: b.Address == f.Active, SignedOut: b.SignedOut}
	}
	return out, nil
}

// Activate makes the Bakery at address the one the window shows.
func (d *Desktop) Activate(address string) error {
	if err := d.store.Activate(address); err != nil {
		return err
	}
	d.events.Emit("bakeries", nil)
	return nil
}

// clientName is how the Bakery's Desktops page names this computer.
func clientName() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "Desktop app"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

// Connect starts a Desktop sign-in at address, opens its approve link in
// the browser (in the window; in `serve` the page shows the link) and polls
// it until it is approved, expired or cancelled. Every change is a
// "connect" event and ConnectStatus's answer.
func (d *Desktop) Connect(address string) (ConnectStart, error) {
	addr, err := bakery.NormalizeAddress(address)
	if err != nil {
		return ConnectStart{}, err
	}
	client := bakery.New(addr, "")
	s, err := client.StartSignIn(context.Background(), clientName())
	if err != nil {
		return ConnectStart{}, fmt.Errorf("could not reach the Bakery at %s: %w", addr, err)
	}
	ctx, stop := context.WithDeadline(context.Background(), s.ExpiresAt.Add(30*time.Second))
	d.mu.Lock()
	d.lastConnect++
	c := &connecting{state: ConnectState{ID: d.lastConnect, Address: addr, Status: connectPending}, client: client, signIn: s, stop: stop}
	d.connects[c.state.ID] = c
	d.mu.Unlock()
	go d.poll(ctx, c)
	if d.openURL != nil {
		// Not opening is no failure: the page shows the link as well.
		_ = d.openURL(s.ApprovalURL)
	}
	return ConnectStart{ID: c.state.ID, Address: addr, ApprovalURL: s.ApprovalURL, ExpiresAt: s.ExpiresAt}, nil
}

// errConnectUnknown is a connect id this Desktop did not start.
var errConnectUnknown = errors.New("no such connect")

// ConnectStatus is where the connect id stands.
func (d *Desktop) ConnectStatus(id uint64) (ConnectState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, ok := d.connects[id]
	if !ok {
		return ConnectState{}, errConnectUnknown
	}
	return c.state, nil
}

// CancelConnect cancels a pending connect on the Bakery and stops polling.
func (d *Desktop) CancelConnect(id uint64) error {
	d.mu.Lock()
	c, ok := d.connects[id]
	d.mu.Unlock()
	if !ok {
		return errConnectUnknown
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Refused when it was approved or expired meanwhile: the poll settles
	// those.
	if err := c.client.CancelSignIn(ctx, c.signIn); err != nil {
		return err
	}
	c.stop()
	d.settle(c, connectCancelled, "")
	return nil
}

// settle records a connect's status, once it is no longer pending, and
// tells the frontend.
func (d *Desktop) settle(c *connecting, status, message string) {
	d.mu.Lock()
	if c.state.Status != connectPending {
		d.mu.Unlock()
		return
	}
	c.state.Status, c.state.Error = status, message
	state := c.state
	d.mu.Unlock()
	d.events.Emit("connect", state)
	if status == connectApproved {
		d.events.Emit("bakeries", nil)
	}
}

// poll asks for the sign-in's status every poll interval, as Paperclip's
// CLI does, until it leaves pending or ctx ends. A failed request is tried
// again on the next tick: the network may come back before it expires.
func (d *Desktop) poll(ctx context.Context, c *connecting) {
	defer c.stop()
	tick := time.NewTicker(c.signIn.PollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			d.settle(c, connectExpired, "")
			return
		case <-tick.C:
		}
		status, err := c.client.SignInStatus(ctx, c.signIn)
		switch {
		case errors.Is(err, bakery.ErrNotFound):
			d.settle(c, connectFailed, "the Bakery no longer knows this sign-in")
			return
		case err != nil:
			continue
		case status == connectPending:
			continue
		case status == connectApproved:
			if err := d.keep(ctx, c.client.Address, c.signIn.Key); err != nil {
				d.settle(c, connectFailed, err.Error())
				return
			}
			d.settle(c, connectApproved, "")
			return
		default:
			d.settle(c, status, "")
			return
		}
	}
}

// keep stores an approved Desktop key with the person and Desktop it
// stands for.
func (d *Desktop) keep(ctx context.Context, address, key string) error {
	keyed := bakery.New(address, key)
	me, err := keyed.Me(ctx)
	if err != nil {
		return fmt.Errorf("approved, but reading who you are failed: %w", err)
	}
	id, err := keyed.DesktopID(ctx)
	if err != nil {
		return fmt.Errorf("approved, but finding this desktop failed: %w", err)
	}
	return d.store.Put(store.Bakery{
		Address:     address,
		DesktopID:   id,
		Key:         key,
		Member:      store.Member{ID: me.ID, Name: me.Name, Email: me.Email},
		ConnectedAt: time.Now().UTC(),
	})
}

// Disconnect signs this Desktop out of the Bakery at address and forgets
// its key, also when the sign-out fails (the Bakery unreachable, the key
// already signed out), as Paperclip's logout does.
func (d *Desktop) Disconnect(address string) error {
	f, err := d.store.Load()
	if err != nil {
		return err
	}
	b, ok := f.Find(address)
	if !ok {
		return store.ErrUnknown
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = bakery.New(b.Address, b.Key).SignOut(ctx)
	if err := d.store.Remove(address); err != nil {
		return err
	}
	d.events.Emit("bakeries", nil)
	return nil
}

// signedOut records a 401 from the Bakery at address and tells the
// frontend; every later call with that key is refused the same way.
func (d *Desktop) signedOut(address string) {
	if err := d.store.MarkSignedOut(address); err == nil {
		d.events.Emit("bakeries", nil)
	}
}
