package app

import (
	"context"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

func TestTickHeartbeats(t *testing.T) {
	ctx := context.Background()
	s, store, _, w := newTest()
	runs := s.runs.(*fakeRuns)
	start := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	clock := start
	s.now = func() time.Time { return clock }
	tick := func(after time.Duration) int {
		t.Helper()
		clock = start.Add(after)
		n, err := s.TickHeartbeats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	ada := hired(t, s, "Ada", 0)
	on, every := true, 60
	if _, err := s.Edit(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, domain.Patch{Heartbeat: &domain.HeartbeatPatch{Enabled: &on, IntervalSec: &every}}); err != nil {
		t.Fatal(err)
	}
	queued := func() []domain.Run {
		var out []domain.Run
		for _, r := range runs.rows {
			if r.Status == domain.RunQueued {
				out = append(out, r)
			}
		}
		return out
	}

	if n := tick(30 * time.Second); n != 0 {
		t.Fatalf("not due yet: %d", n)
	}
	// Due, but without an open Issue: skipped and still due.
	if n := tick(61 * time.Second); n != 0 || store.rows[ada.ID].LastHeartbeatAt != nil {
		t.Fatalf("no open issue: %d %v", n, store.rows[ada.ID].LastHeartbeatAt)
	}
	w.issues = map[uint64]IssueBrief{30: {ID: 30, Identifier: "BAK-3", Status: "todo", AgentAssigneeID: ada.ID}}
	if n := tick(62 * time.Second); n != 1 {
		t.Fatalf("due: %d", n)
	}
	q := queued()
	if len(q) != 1 || q[0].InvocationSource != domain.Timer || q[0].WakeReason != domain.HeartbeatTimer || q[0].IssueID != 0 || q[0].RequestedByID != 7 {
		t.Fatalf("timer run: %+v", q)
	}
	if last := store.rows[ada.ID].LastHeartbeatAt; last == nil || !last.Equal(start.Add(62*time.Second)) {
		t.Fatalf("last heartbeat: %v", last)
	}
	if len(w.actions) != 2 { // hired and approved only: the timer records nothing
		t.Errorf("actions %v", w.actions)
	}
	if n := tick(100 * time.Second); n != 0 {
		t.Fatalf("within the interval: %d", n)
	}
	// The next interval joins the Run still queued.
	if n := tick(122 * time.Second); n != 1 {
		t.Fatalf("next interval: %d", n)
	}
	if q := queued(); len(q) != 1 || q[0].WakeCount != 2 {
		t.Fatalf("joined: %+v", q)
	}
	// Two processes that read the same due Agent: only one claim hits.
	clock = start.Add(200 * time.Second)
	due, _ := store.DueHeartbeats(ctx, clock)
	if len(due) != 1 {
		t.Fatalf("due: %+v", due)
	}
	first, err := s.tickHeartbeat(ctx, due[0], clock)
	second, err2 := s.tickHeartbeat(ctx, due[0], clock)
	if !first || second || err != nil || err2 != nil {
		t.Fatalf("racing ticks: %v %v %v %v", first, second, err, err2)
	}
	if q := queued(); len(q) != 1 || q[0].WakeCount != 3 {
		t.Fatalf("after the race: %+v", q)
	}
	// A done Issue leaves nothing open; a paused Agent is not due.
	w.issues[30] = IssueBrief{ID: 30, Status: "done", AgentAssigneeID: ada.ID}
	if n := tick(300 * time.Second); n != 0 {
		t.Fatalf("done issue: %d", n)
	}
	w.issues[30] = IssueBrief{ID: 30, Status: "todo", AgentAssigneeID: ada.ID}
	if _, err := s.Pause(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID); err != nil {
		t.Fatal(err)
	}
	if n := tick(400 * time.Second); n != 0 {
		t.Fatalf("paused: %d", n)
	}
}
