package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

type fakeStore struct {
	mu         sync.Mutex
	channels   []domain.Channel
	deliveries []domain.Delivery
}

func (f *fakeStore) Channels(context.Context) ([]domain.Channel, error) { return f.channels, nil }
func (f *fakeStore) Channel(_ context.Context, id uint64) (domain.Channel, bool, error) {
	for _, c := range f.channels {
		if c.ID == id {
			return c, true, nil
		}
	}
	return domain.Channel{}, false, nil
}
func (f *fakeStore) CreateChannel(_ context.Context, c domain.Channel) (domain.Channel, error) {
	c.ID = uint64(len(f.channels) + 1)
	f.channels = append(f.channels, c)
	return c, nil
}
func (f *fakeStore) SaveChannel(context.Context, domain.Channel) error { return nil }
func (f *fakeStore) DeleteChannel(context.Context, uint64) error       { return nil }
func (f *fakeStore) ChannelNameTaken(context.Context, string, uint64) (bool, error) {
	return false, nil
}
func (f *fakeStore) CreateDeliveries(_ context.Context, ds []domain.Delivery) ([]domain.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range ds {
		ds[i].ID = uint64(len(f.deliveries) + 1)
		f.deliveries = append(f.deliveries, ds[i])
	}
	return ds, nil
}
func (f *fakeStore) ClaimDue(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Delivery
	for i, d := range f.deliveries {
		if d.Status == domain.Pending && !d.NextAttemptAt.After(now) && len(out) < limit {
			f.deliveries[i].NextAttemptAt = now.Add(lease)
			out = append(out, f.deliveries[i])
		}
	}
	return out, nil
}
func (f *fakeStore) SaveDelivery(_ context.Context, d domain.Delivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliveries[d.ID-1] = d
	return nil
}
func (f *fakeStore) Deliveries(context.Context, uint64, int) ([]domain.Delivery, error) {
	return f.deliveries, nil
}

// fakeSender fails the channels in failing, and counts sends per channel.
type fakeSender struct {
	mu      sync.Mutex
	failing map[uint64]int
	sent    map[uint64]int
	// last is the channel of the latest send, as the sender saw it.
	last domain.Channel
}

func (f *fakeSender) Send(_ context.Context, c domain.Channel, _ domain.Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[c.ID]++
	f.last = c
	if f.failing[c.ID] > 0 {
		f.failing[c.ID]--
		return errors.New("connection refused")
	}
	return nil
}

func setup(t *testing.T) (*Service, *fakeStore, *fakeSender, *time.Time) {
	t.Helper()
	store := &fakeStore{}
	for _, in := range []domain.Input{
		{Name: "all", Kind: domain.Webhook, Settings: domain.Settings{URL: "http://a/"}},
		{Name: "backups", Kind: domain.Webhook, Settings: domain.Settings{URL: "http://b/"}, EventKinds: []domain.EventKind{domain.BackupFailure}},
	} {
		c, err := domain.NewChannel(in)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = store.CreateChannel(context.Background(), c)
	}
	sender := &fakeSender{failing: map[uint64]int{}, sent: map[uint64]int{}}
	s := NewService(store, sender, nil)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	return s, store, sender, &now
}

func TestNotifyOnlySubscribed(t *testing.T) {
	s, store, sender, _ := setup(t)
	s.Notify(context.Background(), domain.Notification{Kind: domain.DeploymentSuccess})
	if len(store.deliveries) != 0 {
		t.Fatalf("nobody subscribes, got %v", store.deliveries)
	}
	s.Notify(context.Background(), domain.Notification{Kind: domain.DeploymentFailure, Title: "x"})
	if len(store.deliveries) != 1 || store.deliveries[0].ChannelID != 1 {
		t.Fatalf("got %+v", store.deliveries)
	}
	if n := s.DispatchDue(context.Background()); n != 1 || sender.sent[1] != 1 || store.deliveries[0].Status != domain.Sent {
		t.Fatalf("dispatched %d, %+v", n, store.deliveries[0])
	}
}

func TestRetriesThenFails(t *testing.T) {
	s, store, sender, now := setup(t)
	sender.failing[2] = 10
	s.Notify(context.Background(), domain.Notification{Kind: domain.BackupFailure})
	s.DispatchDue(context.Background())
	if d := store.deliveries[1]; d.Status != domain.Pending || d.Attempts != 1 || d.LastError != "connection refused" {
		t.Fatalf("after 1: %+v", d)
	}
	if store.deliveries[0].Status != domain.Sent {
		t.Fatalf("the other channel: %+v", store.deliveries[0])
	}
	// Not due yet.
	if s.DispatchDue(context.Background()) != 0 {
		t.Fatal("retried too early")
	}
	*now = now.Add(10 * time.Second)
	s.DispatchDue(context.Background())
	*now = now.Add(60 * time.Second)
	s.DispatchDue(context.Background())
	if d := store.deliveries[1]; d.Status != domain.Failed || d.Attempts != 3 || sender.sent[2] != 3 {
		t.Fatalf("after 3: %+v, sent %d", d, sender.sent[2])
	}
	*now = now.Add(time.Hour)
	if s.DispatchDue(context.Background()) != 0 {
		t.Fatal("a failed Delivery was tried again")
	}
}

func TestSucceedsOnSecondAttempt(t *testing.T) {
	s, store, sender, now := setup(t)
	sender.failing[1] = 1
	s.Notify(context.Background(), domain.Notification{Kind: domain.ServerUnreachable})
	s.DispatchDue(context.Background())
	*now = now.Add(10 * time.Second)
	s.DispatchDue(context.Background())
	if d := store.deliveries[0]; d.Status != domain.Sent || d.Attempts != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestTestChannelIsOneAttempt(t *testing.T) {
	s, store, sender, _ := setup(t)
	sender.failing[1] = 1
	sendErr, err := s.TestChannel(context.Background(), 1, "")
	if err != nil || sendErr == nil {
		t.Fatalf("sendErr %v, err %v", sendErr, err)
	}
	if d := store.deliveries[0]; d.Status != domain.Failed || d.Attempts != 1 {
		t.Fatalf("%+v", d)
	}
	if sendErr, _ := s.TestChannel(context.Background(), 1, ""); sendErr != nil || store.deliveries[1].Status != domain.Sent {
		t.Fatalf("second test: %v %+v", sendErr, store.deliveries[1])
	}
	if _, err := s.TestChannel(context.Background(), 9, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown channel: %v", err)
	}
}

func TestTestChannelWithoutEvents(t *testing.T) {
	s, store, _, _ := setup(t)
	store.channels[0].EventKinds = nil
	if sendErr, err := s.TestChannel(context.Background(), 1, ""); sendErr != nil || err != nil {
		t.Fatalf("sendErr %v, err %v", sendErr, err)
	}
	if d := store.deliveries[0]; d.Notification.Kind != domain.DeploymentSuccess {
		t.Fatalf("%+v", d)
	}
}

func TestDisabledChannelGetsNothing(t *testing.T) {
	s, store, sender, _ := setup(t)
	store.channels[0].Enabled = false
	s.Notify(context.Background(), domain.Notification{Kind: domain.DeploymentFailure})
	if len(store.deliveries) != 0 {
		t.Fatalf("a disabled channel got %+v", store.deliveries)
	}
	_, err := s.TestChannel(context.Background(), 1, "")
	var fe *domain.FieldError
	if !errors.As(err, &fe) || fe.Field != "enabled" || sender.sent[1] != 0 || len(store.deliveries) != 0 {
		t.Fatalf("test of a disabled channel: %v, sent %d", err, sender.sent[1])
	}
}

func TestTestChannelRecipient(t *testing.T) {
	s, store, sender, _ := setup(t)
	var fe *domain.FieldError
	if _, err := s.TestChannel(context.Background(), 1, "probe@example.test"); !errors.As(err, &fe) || fe.Field != "recipient" {
		t.Fatalf("recipient on a webhook channel: %v", err)
	}
	c, err := domain.NewChannel(domain.Input{Name: "mail", Kind: domain.Email, Settings: domain.Settings{
		Host: "smtp", Port: 25, Security: domain.SecurityNone, From: "b@example.com", To: []string{"ops@example.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	c, _ = store.CreateChannel(context.Background(), c)
	if _, err := s.TestChannel(context.Background(), c.ID, "not an address"); !errors.As(err, &fe) || fe.Field != "recipient" {
		t.Fatalf("bad recipient: %v", err)
	}
	if sendErr, err := s.TestChannel(context.Background(), c.ID, " probe@example.test "); sendErr != nil || err != nil {
		t.Fatalf("%v %v", sendErr, err)
	}
	if to := sender.last.Settings.To; len(to) != 1 || to[0] != "probe@example.test" {
		t.Fatalf("sent to %v", to)
	}
	if sendErr, err := s.TestChannel(context.Background(), c.ID, ""); sendErr != nil || err != nil || sender.last.Settings.To[0] != "ops@example.com" {
		t.Fatalf("without a recipient: %v %v %v", sendErr, err, sender.last.Settings.To)
	}
}
