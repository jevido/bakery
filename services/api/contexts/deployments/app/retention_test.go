package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

func TestPruneImagesKeepsRetention(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	for i := uint64(1); i <= 7; i++ {
		s.store.items = append(s.store.items, domain.Deployment{ID: i, ApplicationID: 1, Status: domain.Finished, Image: fmt.Sprintf("localhost/bakery/whoami:%d", i)})
	}
	reclaimed, err := s.service.PruneImages(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed != 200 || len(s.runtime.gone) != 2 || !s.runtime.gone["localhost/bakery/whoami:1"] || !s.runtime.gone["localhost/bakery/whoami:2"] {
		t.Fatalf("reclaimed %d, gone %v", reclaimed, s.runtime.gone)
	}
	// The oldest retained one still rolls back; a pruned one is refused.
	if _, err := s.service.Rollback(ctx, 3); err != nil {
		t.Fatalf("rollback to 3: %v", err)
	}
	if _, err := s.service.Rollback(ctx, 1); !errors.Is(err, ErrImageGone) {
		t.Fatalf("rollback to 1: %v", err)
	}
}

func TestPruneImagesPerServer(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	remote := &fakeRuntime{running: map[string]bool{}}
	s.servers[7] = remote
	for i := uint64(1); i <= 7; i++ {
		s.store.items = append(s.store.items, domain.Deployment{ID: i, ApplicationID: 1, Status: domain.Finished, Image: fmt.Sprintf("localhost/bakery/whoami:%d", i)})
	}
	for i := uint64(8); i <= 14; i++ {
		s.store.items = append(s.store.items, domain.Deployment{ID: i, ApplicationID: 2, ServerID: 7, Status: domain.Finished, Image: fmt.Sprintf("localhost/bakery/remote:%d", i)})
	}
	if _, err := s.service.PruneImages(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if len(s.runtime.gone) != 0 || len(remote.gone) != 2 || !remote.gone["localhost/bakery/remote:8"] {
		t.Fatalf("local gone %v, remote gone %v", s.runtime.gone, remote.gone)
	}
	// A Rollback checks the Image on the Deployment's own Server.
	if _, err := s.service.Rollback(ctx, 8); !errors.Is(err, ErrImageGone) {
		t.Fatalf("rollback to 8: %v", err)
	}
	if _, err := s.service.Rollback(ctx, 10); err != nil {
		t.Fatalf("rollback to 10: %v", err)
	}
	if got := s.store.items[len(s.store.items)-1]; got.ServerID != 7 {
		t.Fatalf("the rollback runs on server %d", got.ServerID)
	}
}
