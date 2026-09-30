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
	reclaimed, err := s.service.PruneImages(ctx)
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
