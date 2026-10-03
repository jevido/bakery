package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// flakyConnector reaches conn unless down is set.
type flakyConnector struct {
	conn *fakeConnection
	down *bool
}

func (f flakyConnector) Connect(context.Context, domain.Server) (Connection, error) {
	if *f.down {
		return nil, errors.New("dial tcp: connection refused")
	}
	return f.conn, nil
}

func TestProbeAll(t *testing.T) {
	ctx := context.Background()
	down := false
	conn := healthy()
	s := NewService(newMemStore(), fakeKey, flakyConnector{conn: conn, down: &down})
	var heard []HealthChanged
	s.OnHealthChanged = func(_ context.Context, e HealthChanged) { heard = append(heard, e) }
	local, _ := s.EnsureLocal(ctx)

	// Not validated yet: not probed.
	down = true
	_ = s.ProbeAll(ctx)
	_ = s.ProbeAll(ctx)
	if got, _ := s.Get(ctx, local.ID); got.FailedProbes != 0 || len(heard) != 0 {
		t.Fatalf("an unvalidated server was probed: %+v %v", got, heard)
	}

	down = false
	if v, err := s.Validate(ctx, local.ID); err != nil || v.Status != domain.Reachable {
		t.Fatalf("validate: %v %s", err, v.Status)
	}
	down = true
	_ = s.ProbeAll(ctx)
	_ = s.ProbeAll(ctx)
	_ = s.ProbeAll(ctx)
	if len(heard) != 1 || heard[0].Change != domain.BecameUnreachable || heard[0].Reason == "" || heard[0].Server.Name != domain.LocalName {
		t.Fatalf("heard %+v", heard)
	}
	if got, _ := s.Get(ctx, local.ID); got.Status != domain.Unreachable || got.FailedProbes != 3 {
		t.Fatalf("stored %+v", got)
	}

	down = false
	conn.free = 5 << 30
	_ = s.ProbeAll(ctx)
	_ = s.ProbeAll(ctx)
	var changes []domain.HealthChange
	for _, e := range heard[1:] {
		changes = append(changes, e.Change)
	}
	if !slices.Equal(changes, []domain.HealthChange{domain.BecameReachable, domain.BecameHighDiskUsage}) {
		t.Fatalf("changes %v", changes)
	}
	if e := heard[2]; e.DiskUsed != 95<<30 || e.DiskTotal != 100<<30 {
		t.Fatalf("disk %+v", e)
	}
	if got, _ := s.Get(ctx, local.ID); got.Status != domain.Reachable || !got.HighDiskUsage {
		t.Fatalf("stored %+v", got)
	}
}
