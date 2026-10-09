package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewRoutine(t *testing.T) {
	for _, tc := range []struct {
		name, title, description string
		field                    string
	}{
		{"trimmed title", "  Weekly check  ", "", ""},
		{"empty title", "   ", "", "title"},
		{"long title", strings.Repeat("x", MaxTitle+1), "", "title"},
		{"longest title", strings.Repeat("é", MaxTitle), "", ""},
		{"long description", "Check", strings.Repeat("x", MaxRoutineDescription+1), "description"},
		{"longest description", "Check", strings.Repeat("é", MaxRoutineDescription), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRoutine(1, ByMember(7), tc.title, tc.description)
			var fe *FieldError
			if tc.field != "" {
				if !errors.As(err, &fe) || fe.Field != tc.field {
					t.Fatalf("err = %v, want a %s error", err, tc.field)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.Title != strings.TrimSpace(tc.title) || r.Status != ActiveRoutine || r.Priority != Medium ||
				r.ConcurrencyPolicy != CoalesceIfActive || r.CatchUpPolicy != SkipMissed || !r.Draft() {
				t.Fatalf("routine = %+v", r)
			}
		})
	}
}

func TestRoutineStatusMoves(t *testing.T) {
	for _, tc := range []struct {
		from, to RoutineStatus
		ok       bool
	}{
		{ActiveRoutine, PausedRoutine, true},
		{PausedRoutine, ActiveRoutine, true},
		{ActiveRoutine, ArchivedRoutine, true},
		{PausedRoutine, ArchivedRoutine, true},
		{ArchivedRoutine, ActiveRoutine, false},
		{ArchivedRoutine, PausedRoutine, false},
		{ArchivedRoutine, ArchivedRoutine, true},
	} {
		r := Routine{Status: tc.from}
		err := r.SetStatus(tc.to)
		if (err == nil) != tc.ok {
			t.Errorf("%s → %s: %v", tc.from, tc.to, err)
		}
		if tc.ok && r.Status != tc.to {
			t.Errorf("%s → %s left %s", tc.from, tc.to, r.Status)
		}
	}
}

func TestRoutineParse(t *testing.T) {
	if _, err := ParseRoutineStatus("draft"); err == nil {
		t.Error("draft is not a Routine status")
	}
	if _, err := ParseConcurrencyPolicy("queue"); err == nil {
		t.Error("queue is not a Concurrency policy")
	}
	if _, err := ParseCatchUpPolicy("enqueue_missed_with_cap"); err != nil {
		t.Error(err)
	}
}

func TestRoutineRelationsStayInGuild(t *testing.T) {
	r := Routine{GuildID: 1}
	if err := r.ServeGoal(&Goal{ID: 5, GuildID: 2}); err == nil {
		t.Error("served another Guild's Goal")
	}
	if err := r.MoveUnder(&Issue{ID: 6, GuildID: 2}); err == nil {
		t.Error("moved under another Guild's Issue")
	}
	if err := r.MoveUnder(&Issue{ID: 6, GuildID: 1}); err != nil || r.ParentIssueID != 6 {
		t.Errorf("parent = %d, %v", r.ParentIssueID, err)
	}
}

func TestRoutineChanges(t *testing.T) {
	b := Routine{ID: 1, GuildID: 1, ProjectID: 2, Title: "A", Status: ActiveRoutine, AssigneeAgentID: 3}
	a := b
	a.Title, a.Status, a.AssigneeAgentID = "B", PausedRoutine, 0
	e := RoutineChanged{Before: b, After: a}
	ch := e.Changes()
	if len(ch) != 3 || ch["assignee"] == nil || ch["status"] == nil || ch["title"] == nil {
		t.Fatalf("changes = %v", ch)
	}
	act := e.Activity()
	if act.EntityType != RoutineEntity || act.ProjectID != 2 || act.Details["title"] != "B" || act.Action != RoutineUpdatedAction {
		t.Fatalf("activity = %+v", act)
	}
}
