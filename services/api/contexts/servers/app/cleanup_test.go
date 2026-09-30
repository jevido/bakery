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
	var retainedOn []uint64
	s.Retention = func(_ context.Context, serverID uint64) (int64, error) {
		retainedOn = append(retainedOn, serverID)
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
	// Retention runs on Remote servers too, named by their id; the Local
	// server is 0.
	if c, err := s.CleanUp(ctx, remote.ID); err != nil || c.Reclaimed != 100 {
		t.Fatalf("%+v %v", c, err)
	}
	if len(retainedOn) != 2 || retainedOn[0] != 0 || retainedOn[1] != remote.ID || conn.pruned != 2 {
		t.Fatalf("retained on %v pruned %d", retainedOn, conn.pruned)
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
