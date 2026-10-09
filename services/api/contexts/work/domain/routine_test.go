package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
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

func TestRoutineTrigger(t *testing.T) {
	now := time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC)
	active := Routine{ID: 1, GuildID: 1, Status: ActiveRoutine, AssigneeAgentID: 3}
	cases := []struct {
		name         string
		r            Routine
		cron, tz     string
		enabled      bool
		field        string
		wantNext     string
		wantTimezone string
		wantArchived bool
	}{
		{name: "in its time zone", r: active, cron: "0 9 * * 1", tz: "Europe/Amsterdam", enabled: true, wantNext: "2025-10-27T08:00:00Z", wantTimezone: "Europe/Amsterdam"},
		{name: "UTC by default", r: active, cron: "daily", enabled: true, wantNext: "2025-10-22T00:00:00Z", wantTimezone: "UTC"},
		{name: "off", r: active, cron: "daily", enabled: false, wantTimezone: "UTC"},
		{name: "paused", r: Routine{ID: 1, GuildID: 1, Status: PausedRoutine, AssigneeAgentID: 3}, cron: "daily", enabled: true, wantTimezone: "UTC"},
		{name: "a Draft", r: Routine{ID: 1, GuildID: 1, Status: ActiveRoutine}, cron: "daily", enabled: true, wantTimezone: "UTC"},
		{name: "a bad cron", r: active, cron: "nope", enabled: true, field: "trigger.cron_expression"},
		{name: "six fields", r: active, cron: "0 0 * * * *", enabled: true, field: "trigger.cron_expression"},
		{name: "a bad time zone", r: active, cron: "daily", tz: "Mars/Base", enabled: true, field: "trigger.timezone"},
		{name: "the server's zone", r: active, cron: "daily", tz: "Local", enabled: true, field: "trigger.timezone"},
		{name: "archived", r: Routine{ID: 1, GuildID: 1, Status: ArchivedRoutine, AssigneeAgentID: 3}, cron: "daily", enabled: true, wantArchived: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr, err := NewScheduleTrigger(c.r, ByMember(7), "", c.cron, c.tz, c.enabled, now)
			var fe *FieldError
			switch {
			case c.wantArchived:
				if !errors.Is(err, ErrArchivedRoutineTriggers) {
					t.Fatalf("err = %v", err)
				}
			case c.field != "":
				if !errors.As(err, &fe) || fe.Field != c.field {
					t.Fatalf("err = %v, want on %s", err, c.field)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if tr.Timezone != c.wantTimezone {
					t.Errorf("time zone %q", tr.Timezone)
				}
				got := ""
				if tr.NextRunAt != nil {
					got = tr.NextRunAt.Format(time.RFC3339)
				}
				if got != c.wantNext {
					t.Errorf("next run %q, want %q", got, c.wantNext)
				}
			}
		})
	}

	api, err := NewAPITrigger(active, ByMember(7), "  CI  ", true)
	if err != nil || api.Label != "CI" || api.NextRunAt != nil || api.CronExpression != "" {
		t.Fatalf("an api trigger = %+v, %v", api, err)
	}
	var fe *FieldError
	if err := api.Change(active, TriggerSettings{Timezone: ptrTo("UTC")}, now); !errors.As(err, &fe) || fe.Field != "trigger.timezone" {
		t.Fatalf("an api trigger with a time zone: %v", err)
	}
	if _, err := NewAPITrigger(active, ByMember(7), strings.Repeat("x", MaxTriggerLabel+1), true); !errors.As(err, &fe) || fe.Field != "trigger.label" {
		t.Fatalf("a long label: %v", err)
	}

	// Changing the label keeps the Next run; changing the cron counts it again.
	tr, _ := NewScheduleTrigger(active, ByMember(7), "", "daily", "", true, now)
	tr.ID = 1
	later := now.Add(48 * time.Hour)
	if err := tr.Change(active, TriggerSettings{Label: ptrTo("Nightly")}, later); err != nil || tr.NextRunAt.Format(time.RFC3339) != "2025-10-22T00:00:00Z" {
		t.Fatalf("relabelled: %v, %v", tr.NextRunAt, err)
	}
	if err := tr.Change(active, TriggerSettings{CronExpression: ptrTo("hourly")}, later); err != nil || tr.NextRunAt.Format(time.RFC3339) != "2025-10-23T13:00:00Z" {
		t.Fatalf("new cron: %v, %v", tr.NextRunAt, err)
	}
}

func ptrTo[T any](v T) *T { return &v }
