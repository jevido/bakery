package main

import (
	"context"

	"github.com/jevido/bakery/apps/desktop/bakery"
	"github.com/jevido/bakery/apps/desktop/runner"
)

// LocalRuns is the Runs this desktop executes right now, across every
// connected Bakery, for the Agent page's live Transcript and "Runs on this
// desktop" without a round trip to a Bakery: nil when this process runs no
// Runner (`serve --no-runner`).
func (d *Desktop) LocalRuns() []runner.LocalRun {
	if d.runner == nil {
		return nil
	}
	return d.runner.LocalRuns()
}

// Runs lists the Agent id's last Runs in the Guild guildID, newest first.
const runsShown = 20

func (d *Desktop) Runs(address string, guildID, id uint64) ([]bakery.Run, error) {
	return ask(d, address, func(ctx context.Context, c *bakery.Client) ([]bakery.Run, error) {
		return c.Runs(ctx, guildID, id, runsShown)
	})
}

// RunEvents answers the Run id's stored events after seq after, in the
// Guild guildID: a final Run's Transcript, read once.
func (d *Desktop) RunEvents(address string, guildID, id uint64, after int64) ([]bakery.RunEvent, error) {
	return ask(d, address, func(ctx context.Context, c *bakery.Client) ([]bakery.RunEvent, error) {
		return c.RunEvents(ctx, guildID, id, after)
	})
}

// followKey names one Run's stream being followed for the frontend.
type followKey struct {
	address string
	guildID uint64
	runID   uint64
}

// RunEventsUpdate is the `run-events` event: one message off a Run's stream
// this desktop did not claim (one of the person's other connected desktops
// runs it), forwarded for the frontend to grow its Transcript live.
type RunEventsUpdate struct {
	Address string           `json:"address"`
	GuildID uint64           `json:"guild_id"`
	RunID   uint64           `json:"run_id"`
	Kind    string           `json:"kind"` // "event" or "end"
	Event   *bakery.RunEvent `json:"event,omitempty"`
	Status  string           `json:"status,omitempty"`
}

// FollowRun starts following the Run id's stream in the Guild guildID, for
// a Run this desktop did not claim, until UnfollowRun or the Run ends.
// Idempotent: following the same Run again does nothing.
func (d *Desktop) FollowRun(address string, guildID, id uint64) error {
	key := followKey{address, guildID, id}
	d.mu.Lock()
	if _, ok := d.following[key]; ok {
		d.mu.Unlock()
		return nil
	}
	c, err := d.keyed(address)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.following[key] = cancel
	d.mu.Unlock()
	go func() {
		defer func() {
			d.mu.Lock()
			if d.following[key] != nil {
				delete(d.following, key)
			}
			d.mu.Unlock()
		}()
		_ = c.FollowRunStream(ctx, guildID, id, func(u bakery.RunStreamUpdate) {
			update := RunEventsUpdate{Address: address, GuildID: guildID, RunID: id, Kind: u.Kind, Status: u.Status}
			if u.Kind == "event" {
				e := u.Event
				update.Event = &e
			}
			d.events.Emit("run-events", update)
		})
	}()
	return nil
}

// UnfollowRun stops following the Run id's stream started by FollowRun.
func (d *Desktop) UnfollowRun(address string, guildID, id uint64) {
	key := followKey{address, guildID, id}
	d.mu.Lock()
	cancel := d.following[key]
	delete(d.following, key)
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
