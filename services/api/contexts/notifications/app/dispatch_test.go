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
}

func (f *fakeSender) Send(_ context.Context, c domain.Channel, _ domain.Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[c.ID]++
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
		{Name: "backups", Kind: domain.Webhook, Settings: domain.Settings{URL: "http://b/"}, EventKinds: []domain.EventKind{domain.BackupFailed}},
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
	s.Notify(context.Background(), domain.Notification{Kind: domain.DeploymentSucceeded})
	if len(store.deliveries) != 0 {
		t.Fatalf("nobody subscribes, got %v", store.deliveries)
	}
	s.Notify(context.Background(), domain.Notification{Kind: domain.DeploymentFailed, Title: "x"})
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
	s.Notify(context.Background(), domain.Notification{Kind: domain.BackupFailed})
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
	sendErr, err := s.TestChannel(context.Background(), 1)
	if err != nil || sendErr == nil {
		t.Fatalf("sendErr %v, err %v", sendErr, err)
	}
	if d := store.deliveries[0]; d.Status != domain.Failed || d.Attempts != 1 {
		t.Fatalf("%+v", d)
	}
	if sendErr, _ := s.TestChannel(context.Background(), 1); sendErr != nil || store.deliveries[1].Status != domain.Sent {
		t.Fatalf("second test: %v %+v", sendErr, store.deliveries[1])
	}
	if _, err := s.TestChannel(context.Background(), 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown channel: %v", err)
	}
}
