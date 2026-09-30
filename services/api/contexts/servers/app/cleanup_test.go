package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

func TestCleanUp(t *testing.T) {
	ctx := context.Background()
	conn := healthy()
	s := NewService(newMemStore(), fakeKey, fakeConnector{conn: conn})
	retained := 0
	s.Retention = func(context.Context) (int64, error) {
		retained++
		return 90, errors.New("one image failed")
	}
	local, _ := s.EnsureLocal(ctx)
	remote, _ := s.Add(ctx, domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})

	c, err := s.CleanUp(ctx, local.ID)
	if err != nil || c.Reclaimed != 100 || c.At.IsZero() {
		t.Fatalf("%+v %v", c, err)
	}
	if got, _ := s.Get(ctx, local.ID); got.LastCleanup != c {
		t.Fatalf("not recorded: %+v", got.LastCleanup)
	}
	// Remote servers have no Deployments yet: only the dangling prune.
	if c, err := s.CleanUp(ctx, remote.ID); err != nil || c.Reclaimed != 10 {
		t.Fatalf("%+v %v", c, err)
	}
	if retained != 1 || conn.pruned != 2 {
		t.Fatalf("retained %d pruned %d", retained, conn.pruned)
	}

	// CleanUpAll skips Servers that are not Reachable.
	if err := s.CleanUpAll(ctx); err != nil || conn.pruned != 2 {
		t.Fatalf("%v pruned %d", err, conn.pruned)
	}
	if _, err := s.Validate(ctx, remote.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.CleanUpAll(ctx); err != nil || conn.pruned != 3 {
		t.Fatalf("%v pruned %d", err, conn.pruned)
	}
}
