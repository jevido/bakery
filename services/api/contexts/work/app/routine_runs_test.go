package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// runService is routineService with Agent 3's Routine "Weekly check" in
// Project 1, where every Issue with an id in live has a queued Run, and
// Assigned recording the Issues it woke.
func runService(t *testing.T) (*Service, *memRoutines, *memActivity, domain.Routine, map[uint64]bool, *[]uint64) {
	t.Helper()
	s, rs, act := routineService(t)
	live, woken := map[uint64]bool{}, &[]uint64{}
	s.IssuesLive = func(_ context.Context, _ uint64, ids []uint64) (map[uint64]bool, error) {
		out := map[uint64]bool{}
		for _, id := range ids {
			out[id] = live[id]
		}
		return out, nil
	}
	s.Assigned = func(_ context.Context, i domain.Issue, _ uint64) error {
		*woken = append(*woken, i.ID)
		return nil
	}
	r, err := s.CreateRoutine(context.Background(), 1, domain.ByMember(7), RoutineInput{Title: "Weekly check", Description: "Look.", AssigneeAgentID: 3, ProjectID: 1, Priority: "high"}, everyProject)
	if err != nil {
		t.Fatal(err)
	}
	act.actions = nil
	return s, rs, act, r, live, woken
}

func TestRunRoutineCreatesAnExecutionIssue(t *testing.T) {
	ctx := context.Background()
	s, rs, act, r, _, woken := runService(t)
	rr, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.ManualSource, Actor: domain.ByMember(7)})
	if err != nil || rr.Status != domain.RunIssueCreated || rr.LinkedIssueID == 0 || rr.TriggeredBy.MemberID != 7 {
		t.Fatalf("RunRoutine = %+v, %v", rr, err)
	}
	i, _, _ := s.issues.Issue(ctx, rr.LinkedIssueID)
	if i.Title != "Weekly check" || i.Description != "Look." || i.Status != domain.Todo || i.Priority != domain.High ||
		i.AssigneeAgentID != 3 || i.ProjectID != 1 || i.OriginRoutineID != r.ID || i.OriginRoutineRunID != rr.ID || i.CreatedBy.MemberID != 7 {
		t.Fatalf("the Execution Issue = %+v", i)
	}
	if len(*woken) != 1 || (*woken)[0] != i.ID {
		t.Errorf("woken %v", *woken)
	}
	if rs.byID[r.ID].LastTriggeredAt == nil || rs.locked {
		t.Errorf("last triggered %v, still locked %v", rs.byID[r.ID].LastTriggeredAt, rs.locked)
	}
	// A manual Routine run shows as the Issue it created.
	if len(act.actions) != 1 || act.actions[0] != domain.IssueCreatedAction {
		t.Errorf("actions %v", act.actions)
	}
}

func TestRunRoutineConcurrencyPolicies(t *testing.T) {
	ctx := context.Background()
	s, _, _, r, live, _ := runService(t)
	manual := RunRequest{Source: domain.ManualSource, Actor: domain.ByMember(7)}
	first, err := s.RunRoutine(ctx, 1, r.ID, manual)
	if err != nil {
		t.Fatal(err)
	}
	// Without a queued or running Run the open Execution Issue is not
	// live, so a second one is created.
	second, err := s.RunRoutine(ctx, 1, r.ID, manual)
	if err != nil || second.Status != domain.RunIssueCreated || second.LinkedIssueID == first.LinkedIssueID {
		t.Fatalf("not live: %+v, %v", second, err)
	}
	live[second.LinkedIssueID] = true
	coalesced, err := s.RunRoutine(ctx, 1, r.ID, manual)
	if err != nil || coalesced.Status != domain.RunCoalesced || coalesced.LinkedIssueID != second.LinkedIssueID ||
		coalesced.CoalescedIntoRunID != second.ID || coalesced.CompletedAt == nil {
		t.Fatalf("coalesce_if_active: %+v, %v", coalesced, err)
	}
	for _, c := range []struct {
		policy string
		want   domain.RoutineRunStatus
		same   bool
	}{{"skip_if_active", domain.RunSkipped, true}, {"always_enqueue", domain.RunIssueCreated, false}} {
		if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{ConcurrencyPolicy: ptr(c.policy)}, everyProject); err != nil {
			t.Fatal(err)
		}
		rr, err := s.RunRoutine(ctx, 1, r.ID, manual)
		if err != nil || rr.Status != c.want || (rr.LinkedIssueID == second.LinkedIssueID) != c.same {
			t.Errorf("%s: %+v, %v", c.policy, rr, err)
		}
	}
}

func TestRunRoutineRefusals(t *testing.T) {
	ctx := context.Background()
	s, rs, _, r, _, _ := runService(t)
	manual := RunRequest{Source: domain.ManualSource, Actor: domain.ByMember(7)}
	sched, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "schedule", CronExpression: "0 9 * * 1"}, everyProject)
	if err != nil {
		t.Fatal(err)
	}
	api, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "api", Enabled: ptr(false)}, everyProject)
	if err != nil {
		t.Fatal(err)
	}
	var fe *domain.FieldError
	if _, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.APISource, TriggerID: sched.ID}); !errors.As(err, &fe) || fe.Field != "trigger_id" {
		t.Errorf("api on a schedule trigger: %v", err)
	}
	if _, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.APISource, TriggerID: api.ID}); !errors.Is(err, domain.ErrTriggerDisabled) {
		t.Errorf("a disabled trigger: %v", err)
	}
	if _, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.ManualSource, Actor: domain.ByAgent(5)}); !errors.Is(err, ErrNotOwnRoutine) {
		t.Errorf("another Agent: %v", err)
	}
	other, _ := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Other", AssigneeAgentID: 5}, everyProject)
	if _, err := s.RunRoutine(ctx, 1, other.ID, RunRequest{Source: domain.APISource, TriggerID: api.ID}); !errors.Is(err, domain.ErrNotRoutinesTrigger) {
		t.Errorf("another Routine's trigger: %v", err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr("paused")}, everyProject); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.ScheduleSource, TriggerID: sched.ID}); !errors.Is(err, domain.ErrRoutinePaused) {
		t.Errorf("a paused schedule: %v", err)
	}
	if rr, err := s.RunRoutine(ctx, 1, r.ID, manual); err != nil || rr.Status != domain.RunIssueCreated {
		t.Errorf("manual while paused: %+v, %v", rr, err)
	}
	draft, _ := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Draft"}, everyProject)
	if _, err := s.RunRoutine(ctx, 1, draft.ID, manual); !errors.As(err, &fe) || fe.Message != "default agent required" {
		t.Errorf("a Draft: %v", err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr("archived")}, everyProject); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunRoutine(ctx, 1, r.ID, manual); !errors.Is(err, domain.ErrRoutineArchivedRun) {
		t.Errorf("archived: %v", err)
	}
	if _, err := s.RunRoutine(ctx, 2, r.ID, manual); !errors.Is(err, ErrNotFound) {
		t.Errorf("another Guild: %v", err)
	}
	// Refused Routine runs are not recorded: only the manual one is.
	if len(rs.runs) != 1 || rs.locked {
		t.Errorf("runs %+v, locked %v", rs.runs, rs.locked)
	}
}

func TestRunRoutineFailsWithoutAnIssue(t *testing.T) {
	ctx := context.Background()
	s, rs, _, r, _, _ := runService(t)
	// The Agent was terminated without the Routine hearing of it.
	r.AssigneeAgentID = 4
	rs.byID[r.ID] = r
	rr, err := s.RunRoutine(ctx, 1, r.ID, RunRequest{Source: domain.ManualSource, Actor: domain.ByMember(7)})
	if err != nil || rr.Status != domain.RunFailed || rr.LinkedIssueID != 0 || rr.FailureReason == "" || rr.CompletedAt == nil {
		t.Fatalf("a terminated Agent: %+v, %v", rr, err)
	}
}

func TestRoutineRunFollowsItsExecutionIssue(t *testing.T) {
	ctx := context.Background()
	s, rs, _, r, live, _ := runService(t)
	manual := RunRequest{Source: domain.ManualSource, Actor: domain.ByMember(7)}
	rr, _ := s.RunRoutine(ctx, 1, r.ID, manual)
	live[rr.LinkedIssueID] = true
	coalesced, _ := s.RunRoutine(ctx, 1, r.ID, manual)
	ref := domain.Identifier("BAK", int(rr.LinkedIssueID))
	for _, c := range []struct {
		status string
		want   domain.RoutineRunStatus
		reason string
	}{
		{"in_progress", domain.RunIssueCreated, ""},
		{"blocked", domain.RunFailed, "Execution issue blocked"},
		{"todo", domain.RunIssueCreated, ""},
		{"cancelled", domain.RunFailed, "Execution issue cancelled"},
		{"done", domain.RunCompleted, ""},
	} {
		if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, IssuePatch{Status: ptr(c.status)}, everyProject); err != nil {
			t.Fatal(err)
		}
		if got := rs.runs[rr.ID-1]; got.Status != c.want || got.FailureReason != c.reason || (got.CompletedAt == nil) != (c.want == domain.RunIssueCreated) {
			t.Errorf("%s: %+v", c.status, got)
		}
	}
	if got := rs.runs[coalesced.ID-1]; got.Status != domain.RunCoalesced {
		t.Errorf("the coalesced one followed: %+v", got)
	}
	again, _ := s.RunRoutine(ctx, 1, r.ID, manual)
	if again.Status != domain.RunIssueCreated {
		t.Fatalf("after done: %+v", again)
	}
	if err := s.DeleteIssue(ctx, 1, 7, domain.Identifier("BAK", int(again.LinkedIssueID)), everyProject); err != nil {
		t.Fatal(err)
	}
	if got := rs.runs[again.ID-1]; got.Status != domain.RunFailed || got.LinkedIssueID != 0 || got.FailureReason != "Execution issue deleted" {
		t.Errorf("deleted: %+v", got)
	}
}
