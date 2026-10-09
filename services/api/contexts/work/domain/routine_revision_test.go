package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSnapshotOfKeepsTheRoutineAndItsTriggers(t *testing.T) {
	now := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	r, err := NewRoutine(1, ByMember(7), "Check {{repo}}", "Look at it")
	if err != nil {
		t.Fatal(err)
	}
	r.ID, r.ProjectID, r.AssigneeAgentID, r.Priority, r.LatestRevisionNumber = 9, 2, 3, High, 4
	sched, err := NewScheduleTrigger(r, ByMember(7), "Mondays", "0 9 * * 1", "Europe/Amsterdam", true, now)
	if err != nil {
		t.Fatal(err)
	}
	sched.ID = 11
	hook, err := NewWebhookTrigger(r, ByMember(7), "GitHub", "pub123", "top-secret", GitHubHMACSigning, 600, true, now)
	if err != nil {
		t.Fatal(err)
	}
	hook.ID, hook.LastDelivery = 12, &WebhookDelivery{Status: AcceptedDelivery, ReceivedAt: now}

	rev := NewRoutineRevision(r, []RoutineTrigger{sched, hook}, ByAgent(3), "Updated routine", 0, now)
	if rev.Number != 5 || rev.Title != "Check {{repo}}" || rev.Author != ByAgent(3) {
		t.Fatalf("revision = %+v", rev)
	}
	s := rev.Snapshot
	if s.Version != 1 || s.Routine.ID != 9 || s.Routine.ProjectID != 2 || s.Routine.AssigneeAgentID != 3 || s.Routine.Priority != High ||
		len(s.Routine.Variables) != 1 || s.Routine.Variables[0].Name != "repo" {
		t.Fatalf("Snapshot's Routine = %+v", s.Routine)
	}
	if len(s.Triggers) != 2 || s.Triggers[0].ID != 11 || s.Triggers[0].CronExpression != "0 9 * * 1" || s.Triggers[0].Timezone != "Europe/Amsterdam" ||
		s.Triggers[1].PublicID != "pub123" || s.Triggers[1].SigningMode != GitHubHMACSigning || s.Triggers[1].ReplayWindowSec != 600 {
		t.Fatalf("Snapshot's triggers = %+v", s.Triggers)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, never := range []string{"top-secret", "next_run", "delivery", "2026-"} {
		if strings.Contains(string(b), never) {
			t.Fatalf("Snapshot JSON holds %q: %s", never, b)
		}
	}
	for _, key := range []string{`"assignee_agent_id":3`, `"cron_expression":"0 9 * * 1"`, `"signing_mode":"github_hmac"`, `"replay_window_sec":600`, `"default_value"`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("Snapshot JSON lacks %s: %s", key, b)
		}
	}
}

func TestCheckBaseRevision(t *testing.T) {
	r := Routine{LatestRevisionID: 8}
	if err := r.CheckBaseRevision(8); err != nil {
		t.Fatalf("the newest: %v", err)
	}
	if err := r.CheckBaseRevision(7); err != ErrStaleRoutineRevision {
		t.Fatalf("an older one: %v", err)
	}
}
