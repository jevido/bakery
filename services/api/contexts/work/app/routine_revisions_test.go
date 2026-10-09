package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// summaries are the Change summaries of the Routine's revisions, oldest
// first, after checking they are numbered 1, 2, … without gaps.
func summaries(t *testing.T, rs *memRoutines, routineID uint64) []string {
	t.Helper()
	revs, err := rs.RoutineRevisions(context.Background(), routineID, MaxRoutineRevisions)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for n := len(revs) - 1; n >= 0; n-- {
		if want := len(revs) - n; revs[n].Number != want {
			t.Fatalf("revision %d is numbered %d", want, revs[n].Number)
		}
		out = append(out, revs[n].ChangeSummary)
	}
	return out
}

func TestEveryChangeAppendsARoutineRevision(t *testing.T) {
	ctx := context.Background()
	s, rs, _ := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	by := domain.ByMember(7)

	r, err := s.CreateRoutine(ctx, 1, by, RoutineInput{Title: "Weekly check", AssigneeAgentID: 3}, all)
	if err != nil || r.LatestRevisionNumber != 1 || r.LatestRevisionID == 0 {
		t.Fatalf("CreateRoutine = %+v, %v", r, err)
	}
	if r, err = s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{Title: ptr("Renamed")}, all); err != nil || r.LatestRevisionNumber != 2 {
		t.Fatalf("ChangeRoutine = %+v, %v", r, err)
	}
	sched, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "schedule", CronExpression: "0 9 * * 1"}, all)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "webhook"}, all)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeTrigger(ctx, 1, by, sched.ID, domain.TriggerSettings{Label: ptr("Mondays")}, all); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RotateTriggerSecret(ctx, 1, by, hook.ID, all); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTrigger(ctx, 1, by, hook.ID, all); err != nil {
		t.Fatal(err)
	}
	// A Routine run is not a change: it appends nothing.
	if _, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.ManualSource, Actor: by}); err != nil {
		t.Fatal(err)
	}
	if err := s.UnassignAgent(ctx, 1, 3, 7); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Created routine", "Updated routine", "Created schedule trigger", "Created webhook trigger",
		"Updated schedule trigger", "Rotated webhook trigger secret", "Deleted webhook trigger", "Agent terminated",
	}
	if got := summaries(t, rs, r.ID); !slices.Equal(got, want) {
		t.Fatalf("Change summaries %q, want %q", got, want)
	}
	if run := rs.runs[0]; run.RoutineRevisionID != rs.revs[6].ID {
		t.Fatalf("the Routine run ran revision %d, want %d", run.RoutineRevisionID, rs.revs[6].ID)
	}
	newest := rs.revs[len(rs.revs)-1]
	if newest.Author != by || newest.Snapshot.Routine.AssigneeAgentID != 0 || newest.Snapshot.Routine.Title != "Renamed" {
		t.Fatalf("newest revision = %+v", newest)
	}
	if len(newest.Snapshot.Triggers) != 1 || newest.Snapshot.Triggers[0].Label != "Mondays" {
		t.Fatalf("newest Snapshot's triggers = %+v", newest.Snapshot.Triggers)
	}
	if got, err := s.RoutineRevisions(ctx, 1, r.ID, all); err != nil || len(got) != 8 || got[0].Number != 8 {
		t.Fatalf("RoutineRevisions = %d, %v", len(got), err)
	}
	if _, err := s.RoutineRevisions(ctx, 2, r.ID, all); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another Guild's Routine's revisions: %v", err)
	}
}

func TestRefusedChangesAppendNoRevision(t *testing.T) {
	ctx := context.Background()
	s, rs, _ := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	by := domain.ByMember(7)
	r, err := s.CreateRoutine(ctx, 1, by, RoutineInput{Title: "Weekly check"}, all)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{Priority: ptr("urgent-ish")}, all); err == nil {
		t.Fatal("a bad Priority was saved")
	}
	if _, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "schedule", CronExpression: "never"}, all); err == nil {
		t.Fatal("a bad cron expression was saved")
	}
	if got := summaries(t, rs, r.ID); !slices.Equal(got, []string{"Created routine"}) {
		t.Fatalf("Change summaries %q", got)
	}
	if rs.locked {
		t.Fatal("a refused change kept the Routine's lock")
	}
}

func TestStaleBaseRevisionIsRefused(t *testing.T) {
	ctx := context.Background()
	s, rs, _ := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	by := domain.ByMember(7)
	r, err := s.CreateRoutine(ctx, 1, by, RoutineInput{Title: "Weekly check"}, all)
	if err != nil {
		t.Fatal(err)
	}
	base := r.LatestRevisionID
	if r, err = s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{Title: ptr("Ada's"), BaseRevisionID: &base}, all); err != nil {
		t.Fatalf("a save on the newest revision: %v", err)
	}
	var stale *StaleRoutineRevisionError
	_, err = s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{Title: ptr("Bo's"), BaseRevisionID: &base}, all)
	if !errors.As(err, &stale) || !errors.Is(err, domain.ErrStaleRoutineRevision) || stale.Current.LatestRevisionID != r.LatestRevisionID {
		t.Fatalf("a save on an older revision: %v", err)
	}
	if rs.byID[r.ID].Title != "Ada's" || rs.locked {
		t.Fatalf("after the stale save: %+v, locked %v", rs.byID[r.ID], rs.locked)
	}
}
