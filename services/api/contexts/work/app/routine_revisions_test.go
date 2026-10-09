package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

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

type goalsPort = Goals

// memGoals keeps Goals in memory; only Goal is asked of it here.
type memGoals struct {
	goalsPort
	byID map[uint64]domain.Goal
}

func (m memGoals) Goal(_ context.Context, id uint64) (domain.Goal, bool, error) {
	g, ok := m.byID[id]
	return g, ok, nil
}

func TestRestoreRoutineRevision(t *testing.T) {
	ctx := context.Background()
	s, rs, act := routineService(t)
	goals := memGoals{byID: map[uint64]domain.Goal{9: {ID: 9, GuildID: 1}}}
	s.goals = goals
	now := time.Date(2025, 4, 1, 8, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	by := domain.ByMember(7)

	r, err := s.CreateRoutine(ctx, 1, by, RoutineInput{
		Title: "Weekly check {{repo}}", Description: "one\ntwo", AssigneeAgentID: 3, ProjectID: 1, GoalID: 9,
		Variables: []domain.RoutineVariable{{Name: "repo", Type: domain.TextVariable, Default: "bakery"}},
	}, all)
	if err != nil {
		t.Fatal(err)
	}
	sched, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "schedule", Label: "Mondays", CronExpression: "0 9 * * 1"}, all)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "webhook", Label: "GitHub alerts"}, all)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "webhook", Label: "Kept"}, all)
	if err != nil {
		t.Fatal(err)
	}
	target := rs.byID[r.ID].LatestRevisionID // revision 4

	// Since revision 4: renamed, moved, re-prioritised, the variable's
	// default changed, the schedule changed, a webhook deleted, an api
	// trigger added, the kept webhook's secret rotated, the Goal deleted.
	if _, err := s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{
		Title: ptr("Renamed {{repo}}"), Description: ptr("one\nthree"), Priority: ptr("high"), ProjectID: ptr(uint64(2)),
		Variables: &[]domain.RoutineVariable{{Name: "repo", Type: domain.TextVariable, Default: "other"}}, Status: ptr("paused"),
	}, all); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeTrigger(ctx, 1, by, sched.ID, domain.TriggerSettings{Label: ptr("Fridays"), CronExpression: ptr("0 9 * * 5")}, all); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTrigger(ctx, 1, by, hook.ID, all); err != nil {
		t.Fatal(err)
	}
	added, err := s.AddTrigger(ctx, 1, by, r.ID, TriggerInput{Kind: "api"}, all)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := s.RotateTriggerSecret(ctx, 1, by, kept.ID, all)
	if err != nil {
		t.Fatal(err)
	}
	delete(goals.byID, 9)
	gone := rs.byID[r.ID]
	gone.GoalID = 0 // as the database clears it
	rs.byID[r.ID] = gone

	now = now.Add(48 * time.Hour)
	res, err := s.RestoreRoutineRevision(ctx, 1, by, r.ID, target, all)
	if err != nil {
		t.Fatal(err)
	}
	got := res.Routine
	if got.Title != "Weekly check {{repo}}" || got.Description != "one\ntwo" || got.Priority != domain.Medium || got.ProjectID != 1 ||
		got.Status != domain.ActiveRoutine || got.GoalID != 0 || got.Variables[0].Default != "bakery" {
		t.Fatalf("restored Routine = %+v", got)
	}
	if res.Revision.Number != 10 || res.Revision.RestoredFromID != target || res.Revision.ChangeSummary != "Restored from revision 4" ||
		got.LatestRevisionID != res.Revision.ID || res.RestoredFrom.Number != 4 {
		t.Fatalf("new revision = %+v, restored from %+v", res.Revision, res.RestoredFrom)
	}
	ts, _ := rs.Triggers(ctx, []uint64{r.ID})
	if len(ts) != 3 || ts[0].ID != sched.ID || ts[1].ID != hook.ID || ts[2].ID != kept.ID {
		t.Fatalf("restored triggers = %+v", ts)
	}
	for _, tr := range ts {
		if tr.ID == added.ID {
			t.Fatal("the api trigger added since was kept")
		}
	}
	if ts[0].Label != "Mondays" || ts[0].CronExpression != "0 9 * * 1" || ts[0].NextRunAt == nil || !ts[0].NextRunAt.Equal(time.Date(2025, 4, 7, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("restored schedule trigger = %+v", ts[0])
	}
	if ts[1].PublicID == "" || ts[1].PublicID == hook.PublicID || ts[1].Secret == "" || ts[1].Secret == hook.Secret || ts[1].Label != "GitHub alerts" {
		t.Fatalf("recreated webhook trigger = %+v", ts[1])
	}
	if len(res.Recreated) != 1 || res.Recreated[0].ID != hook.ID || res.Recreated[0].Secret != ts[1].Secret {
		t.Fatalf("recreated = %+v", res.Recreated)
	}
	if ts[2].PublicID != kept.PublicID || ts[2].Secret != rotated.Secret {
		t.Fatalf("the kept webhook trigger lost its Public id or secret: %+v", ts[2])
	}
	if !slices.Contains(act.actions, domain.RoutineRevisionRestoredAction) {
		t.Fatalf("activity %v", act.actions)
	}

	var newest *RestoreNewestRoutineError
	if _, err := s.RestoreRoutineRevision(ctx, 1, by, r.ID, res.Revision.ID, all); !errors.As(err, &newest) || newest.Current.LatestRevisionID != res.Revision.ID {
		t.Fatalf("restoring the newest revision: %v", err)
	}
	if _, err := s.RestoreRoutineRevision(ctx, 1, by, r.ID, 999, all); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown revision: %v", err)
	}
	if _, err := s.RestoreRoutineRevision(ctx, 1, domain.ByAgent(5), r.ID, target, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("another Agent's Routine: %v", err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{Status: ptr("archived")}, all); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RestoreRoutineRevision(ctx, 1, by, r.ID, target, all); !errors.Is(err, domain.ErrRestoreArchivedRoutine) {
		t.Fatalf("restoring an archived Routine: %v", err)
	}
	if rs.locked {
		t.Fatal("a refused Restore kept the Routine's lock")
	}
}

func TestRestoreRefusesAnAgentThatCannotBeAssigned(t *testing.T) {
	ctx := context.Background()
	s, rs, _ := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	by := domain.ByMember(7)
	r, err := s.CreateRoutine(ctx, 1, by, RoutineInput{Title: "Weekly check", AssigneeAgentID: 5}, all)
	if err != nil {
		t.Fatal(err)
	}
	first := r.LatestRevisionID
	if _, err := s.ChangeRoutine(ctx, 1, by, r.ID, RoutinePatch{AssigneeAgentID: ptr(uint64(3))}, all); err != nil {
		t.Fatal(err)
	}
	// Agent 5 is terminated since; Agent 3 tries to take its revision.
	agents := s.Agents
	s.Agents = func(ctx context.Context, g uint64, ids []uint64) (map[uint64]AssigneeAgent, error) {
		out, err := agents(ctx, g, ids)
		if a, ok := out[5]; ok {
			a.Terminated = true
			out[5] = a
		}
		return out, err
	}
	if _, err := s.RestoreRoutineRevision(ctx, 1, domain.ByAgent(3), r.ID, first, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("an Agent restoring another Agent's revision: %v", err)
	}
	var fe *domain.FieldError
	if _, err := s.RestoreRoutineRevision(ctx, 1, by, r.ID, first, all); !errors.As(err, &fe) || fe.Field != "assignee_agent_id" {
		t.Fatalf("restoring a terminated Agent: %v", err)
	}
	if got := rs.byID[r.ID]; got.AssigneeAgentID != 3 || got.LatestRevisionNumber != 2 || rs.locked {
		t.Fatalf("after the refused Restore: %+v, locked %v", got, rs.locked)
	}
}
