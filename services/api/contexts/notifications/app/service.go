// Package app holds the notifications use cases: manage Notification
// channels, test them, and deliver Notifications to them.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// ErrNotFound is a Notification channel that does not exist.
var ErrNotFound = errors.New("not found")

// Store keeps Notification channels.
type Store interface {
	Channels(ctx context.Context) ([]domain.Channel, error)
	Channel(ctx context.Context, id uint64) (domain.Channel, bool, error)
	CreateChannel(ctx context.Context, c domain.Channel) (domain.Channel, error)
	SaveChannel(ctx context.Context, c domain.Channel) error
	DeleteChannel(ctx context.Context, id uint64) error
	ChannelNameTaken(ctx context.Context, name string, exceptID uint64) (bool, error)
	CreateDeliveries(ctx context.Context, ds []domain.Delivery) ([]domain.Delivery, error)
	// ClaimDue returns pending Deliveries due at now and postpones them by
	// lease, so no one else sends them meanwhile.
	ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error)
	// SaveDelivery writes an attempt's outcome and prunes the channel's
	// Deliveries to the newest domain.KeptDeliveries.
	SaveDelivery(ctx context.Context, d domain.Delivery) error
	Deliveries(ctx context.Context, channelID uint64, limit int) ([]domain.Delivery, error)
}

type Service struct {
	store  Store
	sender Sender
	// DashboardURL is where links in Notifications point, without a
	// trailing slash.
	DashboardURL string
	Log          func(format string, args ...any)
	Now          func() time.Time
	// Poll is how often the dispatcher looks for due Deliveries without
	// being woken.
	Poll time.Duration
	wake chan struct{}
}

func NewService(store Store, sender Sender) *Service {
	return &Service{store: store, sender: sender, Log: func(string, ...any) {}, Now: time.Now, Poll: 5 * time.Second, wake: make(chan struct{}, 1)}
}

func (s *Service) Channels(ctx context.Context) ([]domain.Channel, error) {
	return s.store.Channels(ctx)
}

func (s *Service) Channel(ctx context.Context, id uint64) (domain.Channel, error) {
	c, found, err := s.store.Channel(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return c, err
}

func (s *Service) checkName(ctx context.Context, c domain.Channel) error {
	taken, err := s.store.ChannelNameTaken(ctx, c.Name, c.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "name", Message: "another channel has this name"}
	}
	return nil
}

func (s *Service) AddChannel(ctx context.Context, in domain.Input) (domain.Channel, error) {
	c, err := domain.NewChannel(in)
	if err != nil {
		return c, err
	}
	if err := s.checkName(ctx, c); err != nil {
		return c, err
	}
	return s.store.CreateChannel(ctx, c)
}

// ChangeChannel changes the channel; empty secret settings keep theirs.
func (s *Service) ChangeChannel(ctx context.Context, id uint64, in domain.Input) (domain.Channel, error) {
	c, err := s.Channel(ctx, id)
	if err != nil {
		return c, err
	}
	if err := c.Change(in); err != nil {
		return c, err
	}
	if err := s.checkName(ctx, c); err != nil {
		return c, err
	}
	return c, s.store.SaveChannel(ctx, c)
}

func (s *Service) DeleteChannel(ctx context.Context, id uint64) error {
	if _, err := s.Channel(ctx, id); err != nil {
		return err
	}
	return s.store.DeleteChannel(ctx, id)
}

// testTimeout bounds a Test notification, which the admin waits for.
const testTimeout = 15 * time.Second

// TestChannel sends a Test notification to the channel at once and records
// it as a Delivery. sendErr is the channel's answer; err is anything else
// that went wrong.
func (s *Service) TestChannel(ctx context.Context, id uint64) (sendErr error, err error) {
	c, err := s.Channel(ctx, id)
	if err != nil {
		return nil, err
	}
	n := domain.Notification{
		Kind:  c.EventKinds[0],
		Title: "Test notification from Bakery",
		Body:  "This is a test of the notification channel \"" + c.Name + "\". If you can read it, it works.",
		Link:  s.DashboardURL,
		At:    s.Now(),
	}
	d := domain.NewDelivery(c.ID, n, s.Now())
	// Not due for the dispatcher while this call sends it.
	d.NextAttemptAt = d.NextAttemptAt.Add(2 * testTimeout)
	ds, err := s.store.CreateDeliveries(ctx, []domain.Delivery{d})
	if err != nil {
		return nil, err
	}
	d = ds[0]
	sctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	sendErr = s.sender.Send(sctx, c, n)
	if sendErr != nil {
		d.FailOnce(sendErr.Error(), s.Now())
	} else {
		d.Succeed(s.Now())
	}
	return sendErr, s.store.SaveDelivery(context.WithoutCancel(ctx), d)
}

// Deliveries are the channel's newest Deliveries, newest first.
func (s *Service) Deliveries(ctx context.Context, channelID uint64) ([]domain.Delivery, error) {
	if _, err := s.Channel(ctx, channelID); err != nil {
		return nil, err
	}
	return s.store.Deliveries(ctx, channelID, domain.KeptDeliveries)
}
