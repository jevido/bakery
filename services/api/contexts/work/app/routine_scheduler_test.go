package app

import (
	"context"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// scheduled is runService's Routine with the Catch-up policy and a
// Schedule trigger on cron in UTC, added at added, under a clock the test
// moves with the returned func.
func scheduled(t *testing.T, policy domain.CatchUpPolicy, cron string, added time.Time) (*Service, *memRoutines, domain.Routine, domain.RoutineTrigger, func(time.Time)) {
	t.Helper()
	s, rs, _, r, _, _ := runService(t)
	now := added
	s.now = func() time.Time { return now }
	r.CatchUpPolicy = policy
	rs.byID[r.ID] = r
	tr, err := s.AddTrigger(context.Background(), 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "schedule", CronExpression: cron, Timezone: "UTC"}, everyProject)
	if err != nil {
		t.Fatal(err)
	}
	return s, rs, r, tr, func(at time.Time) { now = at }
}

func tick(t *testing.T, s *Service) int {
	t.Helper()
	n, err := s.TickRoutines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func nextRun(rs *memRoutines, id uint64) time.Time {
	t, _, _ := rs.Trigger(context.Background(), id)
	if t.NextRunAt == nil {
		return time.Time{}
	}
	return *t.NextRunAt
}

func TestTickFiresADueTriggerOnce(t *testing.T) {
	s, rs, _, tr, at := scheduled(t, domain.SkipMissed, "0 9 * * *", time.Date(2025, 3, 25, 8, 0, 0, 0, time.UTC))
	if n := tick(t, s); n != 0 {
		t.Fatalf("fired %d before it was due", n)
	}
	at(time.Date(2025, 3, 25, 9, 0, 10, 0, time.UTC))
	if n := tick(t, s); n != 1 {
		t.Fatalf("first tick fired %d", n)
	}
	at(time.Date(2025, 3, 25, 9, 0, 25, 0, time.UTC))
	if n := tick(t, s); n != 0 {
		t.Fatalf("second tick fired %d", n)
	}
	if len(rs.runs) != 1 || rs.runs[0].Source != domain.ScheduleSource || rs.runs[0].TriggerID != tr.ID || !rs.runs[0].TriggeredBy.None() {
		t.Fatalf("routine runs %+v", rs.runs)
	}
	if got := nextRun(rs, tr.ID); !got.Equal(time.Date(2025, 3, 26, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("next run %v", got)
	}
	got, _, _ := rs.Trigger(context.Background(), tr.ID)
	if got.LastFiredAt == nil || got.LastResult != string(domain.RunIssueCreated) {
		t.Errorf("trigger after firing %+v", got)
	}
}

// claimedElsewhere moves a trigger's Next run between DueTriggers and
// ClaimTrigger, as another API process claiming it first would.
type claimedElsewhere struct{ *memRoutines }

func (c claimedElsewhere) DueTriggers(ctx context.Context, now time.Time) ([]domain.RoutineTrigger, error) {
	due, err := c.memRoutines.DueTriggers(ctx, now)
	for i := range c.triggers {
		later := c.triggers[i].NextRunAt.Add(24 * time.Hour)
		c.triggers[i].NextRunAt = &later
	}
	return due, err
}

func TestTickLeavesATriggerClaimedElsewhere(t *testing.T) {
	s, rs, _, _, at := scheduled(t, domain.SkipMissed, "0 9 * * *", time.Date(2025, 3, 25, 8, 0, 0, 0, time.UTC))
	s.routines = claimedElsewhere{rs}
	at(time.Date(2025, 3, 25, 9, 0, 10, 0, time.UTC))
	if n := tick(t, s); n != 0 || len(rs.runs) != 0 {
		t.Fatalf("fired %d, routine runs %+v", n, rs.runs)
	}
}

func TestSkipMissedFiresOnce(t *testing.T) {
	s, rs, _, tr, at := scheduled(t, domain.SkipMissed, "0 * * * *", time.Date(2025, 3, 25, 8, 30, 0, 0, time.UTC))
	at(time.Date(2025, 3, 25, 11, 30, 0, 0, time.UTC)) // 09:00, 10:00 and 11:00 missed
	if n := tick(t, s); n != 1 {
		t.Fatalf("fired %d", n)
	}
	if got := nextRun(rs, tr.ID); !got.Equal(time.Date(2025, 3, 25, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("next run %v", got)
	}
}

func TestEnqueueMissedWithCap(t *testing.T) {
	s, rs, r, tr, at := scheduled(t, domain.EnqueueMissedWithCap, "0 9 * * *", time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC))
	r.ConcurrencyPolicy = domain.AlwaysEnqueue
	rs.byID[r.ID] = r
	at(time.Date(2025, 3, 3, 12, 0, 0, 0, time.UTC)) // the 1st, 2nd and 3rd missed
	if n := tick(t, s); n != 3 {
		t.Fatalf("fired %d of 3 missed ticks", n)
	}
	if got := nextRun(rs, tr.ID); !got.Equal(time.Date(2025, 3, 4, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("next run %v", got)
	}

	at(time.Date(2025, 4, 2, 12, 0, 0, 0, time.UTC)) // the 4th to the 2nd of April: 30 missed
	if n := tick(t, s); n != domain.MaxCatchUpRuns {
		t.Fatalf("fired %d of 30 missed ticks", n)
	}
	// The cap leaves the Next run at the 26th missed tick, made up next.
	if got := nextRun(rs, tr.ID); !got.Equal(time.Date(2025, 3, 29, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("next run after the cap %v", got)
	}
}

func TestEnqueueMissedFiresASubHourlyCronOnce(t *testing.T) {
	s, rs, _, tr, at := scheduled(t, domain.EnqueueMissedWithCap, "*/5 * * * *", time.Date(2025, 3, 25, 8, 1, 0, 0, time.UTC))
	at(time.Date(2025, 3, 25, 9, 2, 0, 0, time.UTC)) // twelve missed
	if n := tick(t, s); n != 1 {
		t.Fatalf("fired %d", n)
	}
	if got := nextRun(rs, tr.ID); !got.Equal(time.Date(2025, 3, 25, 9, 5, 0, 0, time.UTC)) {
		t.Errorf("next run %v", got)
	}
}

func TestTickLeavesAPausedRoutine(t *testing.T) {
	s, rs, r, tr, at := scheduled(t, domain.SkipMissed, "0 9 * * *", time.Date(2025, 3, 25, 8, 0, 0, 0, time.UTC))
	// Paused behind the service's back, so the Next run is still set.
	r.Status = domain.PausedRoutine
	rs.byID[r.ID] = r
	at(time.Date(2025, 3, 25, 9, 0, 10, 0, time.UTC))
	if n := tick(t, s); n != 0 || len(rs.runs) != 0 {
		t.Fatalf("fired %d", n)
	}
	if got := nextRun(rs, tr.ID); !got.Equal(time.Date(2025, 3, 25, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("next run %v", got)
	}
}
