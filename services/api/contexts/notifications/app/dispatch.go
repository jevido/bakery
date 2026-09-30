package app

import (
	"context"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

const (
	// attemptTimeout bounds one attempt of one Delivery.
	attemptTimeout = 15 * time.Second
	// lease is how long a claimed Delivery is left alone by other claims;
	// longer than an attempt, so it only matters after a crash.
	lease = 2 * time.Minute
	// batch is how many due Deliveries are claimed at once, and sending is
	// how many of them are sent at the same time.
	batch   = 20
	sending = 4
)

// Notify records a Delivery of n for every channel subscribed to its Event
// kind and wakes the dispatcher. It never fails the caller: what goes
// wrong is logged.
func (s *Service) Notify(ctx context.Context, n domain.Notification) {
	if n.At.IsZero() {
		n.At = s.Now()
	}
	channels, err := s.store.Channels(ctx)
	if err != nil {
		s.Log("notifications: %s: reading channels: %v", n.Kind, err)
		return
	}
	var ds []domain.Delivery
	for _, c := range channels {
		if c.Subscribed(n.Kind) {
			ds = append(ds, domain.NewDelivery(c.ID, n, s.Now()))
		}
	}
	if len(ds) == 0 {
		return
	}
	if _, err := s.store.CreateDeliveries(ctx, ds); err != nil {
		s.Log("notifications: %s: storing deliveries: %v", n.Kind, err)
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run sends due Deliveries until ctx ends: at once when Notify wakes it,
// else every Poll. Deliveries left pending by an earlier process are due
// too, so a restart resumes them.
func (s *Service) Run(ctx context.Context) {
	for {
		for s.DispatchDue(ctx) == batch {
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-time.After(s.Poll):
		}
	}
}

// DispatchDue claims and sends one batch of due Deliveries, returning how
// many it claimed.
func (s *Service) DispatchDue(ctx context.Context) int {
	due, err := s.store.ClaimDue(ctx, s.Now(), lease, batch)
	if err != nil {
		if ctx.Err() == nil {
			s.Log("notifications: claiming deliveries: %v", err)
		}
		return 0
	}
	sem := make(chan struct{}, sending)
	var wg sync.WaitGroup
	for _, d := range due {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.attempt(ctx, d)
		}()
	}
	wg.Wait()
	return len(due)
}

func (s *Service) attempt(ctx context.Context, d domain.Delivery) {
	c, found, err := s.store.Channel(ctx, d.ChannelID)
	if err != nil {
		s.Log("notifications: delivery %d: reading channel: %v", d.ID, err)
		return
	}
	if !found {
		// Deleted meanwhile, and its Deliveries with it.
		return
	}
	actx, cancel := context.WithTimeout(ctx, attemptTimeout)
	err = s.sender.Send(actx, c, d.Notification)
	cancel()
	if err != nil && ctx.Err() != nil {
		// Shutting down: not the channel's fault. The lease runs out and
		// the next process tries again.
		return
	}
	if err != nil {
		d.Fail(err.Error(), s.Now())
		if d.Status == domain.Failed {
			s.Log("notifications: %s to channel %q failed after %d attempts: %v", d.Notification.Kind, c.Name, d.Attempts, err)
		}
	} else {
		d.Succeed(s.Now())
	}
	// The outcome is stored even when shutting down.
	sctx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelSave()
	if err := s.store.SaveDelivery(sctx, d); err != nil {
		s.Log("notifications: saving delivery %d: %v", d.ID, err)
	}
}
