package app

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// memRoutines keeps Routines in memory.
type memRoutines struct {
	byID     map[uint64]domain.Routine
	triggers []domain.RoutineTrigger
	runs     []domain.RoutineRun
	locked   bool
}

func (m *memRoutines) Routines(_ context.Context, guildID uint64) ([]domain.Routine, error) {
	var out []domain.Routine
	for id := uint64(len(m.byID)); id > 0; id-- {
		if r, ok := m.byID[id]; ok && r.GuildID == guildID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memRoutines) Routine(_ context.Context, id uint64) (domain.Routine, bool, error) {
	r, ok := m.byID[id]
	return r, ok, nil
}

func (m *memRoutines) CreateRoutine(_ context.Context, r domain.Routine) (domain.Routine, error) {
	r.ID = uint64(len(m.byID) + 1)
	m.byID[r.ID] = r
	return r, nil
}

func (m *memRoutines) SaveRoutine(_ context.Context, r domain.Routine) error {
	m.byID[r.ID] = r
	return nil
}

func (m *memRoutines) RoutinesOfAgent(_ context.Context, guildID, agentID uint64) ([]domain.Routine, error) {
	var out []domain.Routine
	for _, r := range m.byID {
		if r.GuildID == guildID && r.AssigneeAgentID == agentID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memRoutines) DeleteRoutinesOfProject(_ context.Context, projectID uint64) error {
	for id, r := range m.byID {
		if r.ProjectID == projectID {
			delete(m.byID, id)
		}
	}
	return nil
}

func (m *memRoutines) Triggers(_ context.Context, routineIDs []uint64) ([]domain.RoutineTrigger, error) {
	var out []domain.RoutineTrigger
	for _, t := range m.triggers {
		if slices.Contains(routineIDs, t.RoutineID) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *memRoutines) Trigger(_ context.Context, id uint64) (domain.RoutineTrigger, bool, error) {
	for _, t := range m.triggers {
		if t.ID == id {
			return t, true, nil
		}
	}
	return domain.RoutineTrigger{}, false, nil
}

func (m *memRoutines) TriggerByPublicID(_ context.Context, publicID string) (domain.RoutineTrigger, bool, error) {
	for _, t := range m.triggers {
		if t.PublicID != "" && t.PublicID == publicID {
			return t, true, nil
		}
	}
	return domain.RoutineTrigger{}, false, nil
}

func (m *memRoutines) CreateTrigger(_ context.Context, t domain.RoutineTrigger) (domain.RoutineTrigger, error) {
	t.ID = uint64(len(m.triggers) + 1)
	m.triggers = append(m.triggers, t)
	return t, nil
}

func (m *memRoutines) SaveTrigger(_ context.Context, t domain.RoutineTrigger) error {
	for i := range m.triggers {
		if m.triggers[i].ID == t.ID {
			m.triggers[i] = t
		}
	}
	return nil
}

func (m *memRoutines) LockRoutine(context.Context, uint64) (func(), error) {
	if m.locked {
		return nil, errors.New("already locked")
	}
	m.locked = true
	return func() { m.locked = false }, nil
}

func (m *memRoutines) RoutineTriggered(_ context.Context, id uint64, at time.Time) error {
	r := m.byID[id]
	r.LastTriggeredAt = &at
	m.byID[id] = r
	return nil
}

func (m *memRoutines) CreateRoutineRun(_ context.Context, rr domain.RoutineRun) (domain.RoutineRun, error) {
	rr.ID = uint64(len(m.runs) + 1)
	m.runs = append(m.runs, rr)
	return rr, nil
}

func (m *memRoutines) SaveRoutineRun(_ context.Context, rr domain.RoutineRun) error {
	m.runs[rr.ID-1] = rr
	return nil
}

func (m *memRoutines) RoutineRun(_ context.Context, id uint64) (domain.RoutineRun, bool, error) {
	if id == 0 || id > uint64(len(m.runs)) {
		return domain.RoutineRun{}, false, nil
	}
	return m.runs[id-1], true, nil
}

func (m *memRoutines) RoutineRunByIdempotencyKey(_ context.Context, triggerID uint64, key string) (domain.RoutineRun, bool, error) {
	for _, rr := range m.runs {
		if rr.TriggerID == triggerID && rr.IdempotencyKey == key {
			return rr, true, nil
		}
	}
	return domain.RoutineRun{}, false, nil
}

func (m *memRoutines) RoutineRuns(_ context.Context, routineIDs []uint64, limit int) ([]domain.RoutineRun, error) {
	var out []domain.RoutineRun
	for n := len(m.runs) - 1; n >= 0 && len(out) < limit; n-- {
		if slices.Contains(routineIDs, m.runs[n].RoutineID) {
			out = append(out, m.runs[n])
		}
	}
	return out, nil
}

func (m *memRoutines) LastRoutineRuns(_ context.Context, routineIDs []uint64) (map[uint64]domain.RoutineRun, error) {
	out := map[uint64]domain.RoutineRun{}
	for _, rr := range m.runs {
		if slices.Contains(routineIDs, rr.RoutineID) {
			out[rr.RoutineID] = rr
		}
	}
	return out, nil
}

func (m *memRoutines) DeleteTrigger(_ context.Context, id uint64) error {
	m.triggers = slices.DeleteFunc(m.triggers, func(t domain.RoutineTrigger) bool { return t.ID == id })
	return nil
}

// DueTriggers is the infra query over memory: it reads the Routines too.
func (m *memRoutines) DueTriggers(_ context.Context, now time.Time) ([]domain.RoutineTrigger, error) {
	var out []domain.RoutineTrigger
	for _, t := range m.triggers {
		r := m.byID[t.RoutineID]
		if t.Kind == domain.ScheduleTrigger && t.Enabled && r.Status == domain.ActiveRoutine && r.AssigneeAgentID != 0 &&
			t.NextRunAt != nil && !t.NextRunAt.After(now) {
			out = append(out, t)
		}
	}
	slices.SortStableFunc(out, func(a, b domain.RoutineTrigger) int { return a.NextRunAt.Compare(*b.NextRunAt) })
	return out, nil
}

func (m *memRoutines) ClaimTrigger(_ context.Context, id uint64, seen, next time.Time) (bool, error) {
	for i, t := range m.triggers {
		if t.ID == id && t.Enabled && t.NextRunAt != nil && t.NextRunAt.Equal(seen) {
			m.triggers[i].NextRunAt = &next
			return true, nil
		}
	}
	return false, nil
}

// routineService has Agents 3 and 5, and Agent 4, which is terminated.
func routineService(t *testing.T) (*Service, *memRoutines, *memActivity) {
	t.Helper()
	rs, act := &memRoutines{byID: map[uint64]domain.Routine{}}, &memActivity{}
	s := NewService(nil, &memIssues{byID: map[uint64]domain.Issue{}}, nil, nil, memberGuild{}, memProjects{}, act, nil, nil, rs)
	s.Logf = t.Logf
	s.Agents = func(_ context.Context, _ uint64, ids []uint64) (map[uint64]AssigneeAgent, error) {
		out := map[uint64]AssigneeAgent{}
		for _, id := range ids {
			if id >= 3 && id <= 5 {
				out[id] = AssigneeAgent{Name: "Builder", Terminated: id == 4}
			}
		}
		return out, nil
	}
	return s, rs, act
}

func TestCreateRoutine(t *testing.T) {
	ctx := context.Background()
	s, _, act := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	var fe *domain.FieldError

	r, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Weekly check", AssigneeAgentID: 3, ProjectID: 1, Priority: "high"}, all)
	if err != nil || r.AssigneeAgentID != 3 || r.ProjectID != 1 || r.Priority != domain.High || r.Status != domain.ActiveRoutine {
		t.Fatalf("CreateRoutine = %+v, %v", r, err)
	}
	if d, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Draft"}, all); err != nil || !d.Draft() {
		t.Fatalf("a Draft = %+v, %v", d, err)
	}
	if _, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Gone", AssigneeAgentID: 4}, all); !errors.As(err, &fe) || fe.Field != "assignee_agent_id" {
		t.Fatalf("a terminated Agent: %v", err)
	}
	if _, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Nowhere", ProjectID: 9}, all); !errors.As(err, &fe) || fe.Field != "project_id" {
		t.Fatalf("an unknown Project: %v", err)
	}
	none := func([]uint64) ([]uint64, error) { return nil, nil }
	if _, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Hidden", ProjectID: 1}, none); !errors.As(err, &fe) || fe.Field != "project_id" {
		t.Fatalf("a Project the person may not view: %v", err)
	}
	if _, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Bad", ConcurrencyPolicy: "queue"}, all); !errors.As(err, &fe) || fe.Field != "concurrency_policy" {
		t.Fatalf("a bad Concurrency policy: %v", err)
	}
	if !slices.Equal(act.actions, []string{domain.RoutineCreatedAction, domain.RoutineCreatedAction}) {
		t.Fatalf("recorded %v", act.actions)
	}
	if got, err := s.Routines(ctx, 1, RoutineFilter{}, none); err != nil || len(got) != 1 || got[0].ProjectID != 0 {
		t.Fatalf("a list without the Project's Routines = %+v, %v", got, err)
	}
	if _, err := s.Routine(ctx, 1, r.ID, none); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a Routine in a hidden Project: %v", err)
	}
	if _, err := s.Routine(ctx, 2, r.ID, all); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another Guild's Routine: %v", err)
	}
}

func TestChangeRoutineStatus(t *testing.T) {
	ctx := context.Background()
	s, _, act := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	r, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Weekly check", AssigneeAgentID: 3}, all)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range []string{"paused", "active", "archived"} {
		if r, err = s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr(st)}, all); err != nil || string(r.Status) != st {
			t.Fatalf("to %s = %+v, %v", st, r, err)
		}
	}
	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr("active")}, all); !errors.Is(err, domain.ErrRoutineArchived) {
		t.Fatalf("leaving archived: %v", err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Title: ptr("Renamed")}, all); !errors.Is(err, domain.ErrRoutineArchived) {
		t.Fatalf("changing an archived Routine: %v", err)
	}
	want := []string{domain.RoutineCreatedAction, domain.RoutineUpdatedAction, domain.RoutineUpdatedAction, domain.RoutineArchivedAction}
	if !slices.Equal(act.actions, want) {
		t.Fatalf("recorded %v, want %v", act.actions, want)
	}
}

func TestAgentManagesOnlyItsOwnRoutines(t *testing.T) {
	ctx := context.Background()
	s, _, _ := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	agent := domain.ByAgent(3)
	if _, err := s.CreateRoutine(ctx, 1, agent, RoutineInput{Title: "For another", AssigneeAgentID: 5}, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("an Agent created another's Routine: %v", err)
	}
	own, err := s.CreateRoutine(ctx, 1, agent, RoutineInput{Title: "Mine", AssigneeAgentID: 3}, all)
	if err != nil || own.CreatedBy.AgentID != 3 {
		t.Fatalf("own Routine = %+v, %v", own, err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, agent, own.ID, RoutinePatch{AssigneeAgentID: ptr(uint64(5))}, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("an Agent handed its Routine on: %v", err)
	}
	other, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Theirs", AssigneeAgentID: 5}, all)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, agent, other.ID, RoutinePatch{Title: ptr("Taken")}, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("an Agent changed another's Routine: %v", err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, agent, own.ID, RoutinePatch{Status: ptr("paused")}, all); err != nil {
		t.Fatalf("an Agent pausing its own Routine: %v", err)
	}
}

func TestTerminatingAnAgentMakesItsRoutinesDrafts(t *testing.T) {
	ctx := context.Background()
	s, rs, act := routineService(t)
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	live, _ := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Live", AssigneeAgentID: 3}, all)
	old, _ := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Old", AssigneeAgentID: 3, Status: "archived"}, all)
	act.actions = nil
	if err := s.UnassignAgent(ctx, 1, 3, 7); err != nil {
		t.Fatal(err)
	}
	if !rs.byID[live.ID].Draft() || rs.byID[old.ID].Draft() {
		t.Fatalf("after terminating: live %+v, archived %+v", rs.byID[live.ID], rs.byID[old.ID])
	}
	if !slices.Equal(act.actions, []string{domain.RoutineUpdatedAction}) {
		t.Fatalf("recorded %v", act.actions)
	}
}

func TestDeletingAProjectDeletesItsRoutines(t *testing.T) {
	ctx := context.Background()
	s, rs, _ := routineService(t)
	s.issues = &leavingIssues{memIssues: memIssues{byID: map[uint64]domain.Issue{}}}
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	in, _ := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "In", ProjectID: 1}, all)
	out, _ := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Out", ProjectID: 2}, all)
	if err := s.ForgetProject(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := rs.byID[in.ID]; ok {
		t.Fatal("the deleted Project's Routine is left")
	}
	if _, ok := rs.byID[out.ID]; !ok {
		t.Fatal("another Project's Routine was deleted")
	}
}

type leavingIssues struct{ memIssues }

func (*leavingIssues) LeaveProject(context.Context, uint64) error { return nil }

func TestRoutineTriggersFollowTheRoutine(t *testing.T) {
	ctx := context.Background()
	s, rs, act := routineService(t)
	now := time.Date(2025, 3, 25, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	r, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Weekly check", AssigneeAgentID: 3}, all)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "schedule", CronExpression: "0 9 * * 1", Timezone: "Europe/Amsterdam"}, all)
	if err != nil || tr.NextRunAt == nil || !tr.NextRunAt.Equal(time.Date(2025, 3, 31, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("AddTrigger = %+v, %v", tr, err)
	}
	api, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "api", Label: "CI"}, all)
	if err != nil || api.NextRunAt != nil {
		t.Fatalf("an api trigger = %+v, %v", api, err)
	}
	var fe *domain.FieldError
	if _, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "api", CronExpression: "daily"}, all); !errors.As(err, &fe) || fe.Field != "trigger.cron_expression" {
		t.Fatalf("an api trigger with a cron: %v", err)
	}
	next := func() *time.Time {
		got, _, _ := rs.Trigger(ctx, tr.ID)
		return got.NextRunAt
	}

	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr("paused")}, all); err != nil || next() != nil {
		t.Fatalf("paused: next run %v, %v", next(), err)
	}
	// Resuming counts from now: the Monday missed while paused never fires.
	now = time.Date(2025, 4, 2, 12, 0, 0, 0, time.UTC)
	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr("active")}, all); err != nil || next() == nil || !next().Equal(time.Date(2025, 4, 7, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("resumed: next run %v, %v", next(), err)
	}
	if _, err := s.ChangeTrigger(ctx, 1, domain.ByMember(7), tr.ID, domain.TriggerSettings{Enabled: ptr(false)}, all); err != nil || next() != nil {
		t.Fatalf("off: next run %v, %v", next(), err)
	}
	if _, err := s.ChangeTrigger(ctx, 1, domain.ByMember(7), tr.ID, domain.TriggerSettings{Enabled: ptr(true)}, all); err != nil || next() == nil {
		t.Fatalf("on again: next run %v, %v", next(), err)
	}
	if err := s.UnassignAgent(ctx, 1, 3, 7); err != nil || next() != nil {
		t.Fatalf("a Draft: next run %v, %v", next(), err)
	}
	if _, err := s.AddTrigger(ctx, 1, domain.ByAgent(5), r.ID, TriggerInput{Kind: "api"}, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("an Agent on another's Routine: %v", err)
	}
	if err := s.DeleteTrigger(ctx, 1, domain.ByMember(7), api.ID, all); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeRoutine(ctx, 1, domain.ByMember(7), r.ID, RoutinePatch{Status: ptr("archived")}, all); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeTrigger(ctx, 1, domain.ByMember(7), tr.ID, domain.TriggerSettings{Label: ptr("x")}, all); !errors.Is(err, domain.ErrArchivedRoutineTriggers) {
		t.Fatalf("changing an archived Routine's trigger: %v", err)
	}
	if err := s.DeleteTrigger(ctx, 1, domain.ByMember(7), tr.ID, all); !errors.Is(err, domain.ErrArchivedRoutineTriggers) {
		t.Fatalf("deleting an archived Routine's trigger: %v", err)
	}
	var got []string
	for _, a := range act.actions {
		if strings.HasPrefix(a, "routine.trigger_") {
			got = append(got, a)
		}
	}
	want := []string{domain.RoutineTriggerCreatedAction, domain.RoutineTriggerCreatedAction, domain.RoutineTriggerUpdatedAction, domain.RoutineTriggerUpdatedAction, domain.RoutineTriggerDeletedAction}
	if !slices.Equal(got, want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
}

func TestWebhookTriggerSecret(t *testing.T) {
	ctx := context.Background()
	s, rs, act := routineService(t)
	now := time.Date(2025, 3, 25, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	r, err := s.CreateRoutine(ctx, 1, domain.ByMember(7), RoutineInput{Title: "Triage", AssigneeAgentID: 3}, all)
	if err != nil {
		t.Fatal(err)
	}
	hex := regexp.MustCompile(`^[0-9a-f]+$`)
	wh, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "webhook", SigningMode: "hmac_sha256"}, all)
	if err != nil || len(wh.Secret) != 48 || !hex.MatchString(wh.Secret) || len(wh.PublicID) != 24 || !hex.MatchString(wh.PublicID) {
		t.Fatalf("AddTrigger = %+v, %v", wh, err)
	}
	if wh.SigningMode != domain.HMACSHA256Signing || wh.ReplayWindowSec != domain.DefaultReplayWindow || wh.NextRunAt != nil {
		t.Fatalf("a webhook trigger = %+v", wh)
	}
	if found, ok, _ := rs.TriggerByPublicID(ctx, wh.PublicID); !ok || found.ID != wh.ID {
		t.Fatalf("by Public id = %+v, %v", found, ok)
	}
	other, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "webhook"}, all)
	if err != nil || other.SigningMode != domain.BearerSigning || other.Secret == wh.Secret || other.PublicID == wh.PublicID {
		t.Fatalf("a second webhook trigger = %+v, %v", other, err)
	}
	var fe *domain.FieldError
	for _, in := range []TriggerInput{
		{Kind: "webhook", SigningMode: "md5"},
		{Kind: "api", SigningMode: "bearer"},
	} {
		if _, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, in, all); !errors.As(err, &fe) || fe.Field != "trigger.signing_mode" {
			t.Fatalf("AddTrigger(%+v): %v", in, err)
		}
	}
	if _, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "webhook", ReplayWindowSec: 10}, all); !errors.As(err, &fe) || fe.Field != "trigger.replay_window_sec" {
		t.Fatalf("a Replay window of 10: %v", err)
	}

	now = now.Add(time.Hour)
	rotated, err := s.RotateTriggerSecret(ctx, 1, domain.ByMember(7), wh.ID, all)
	if err != nil || rotated.Secret == wh.Secret || len(rotated.Secret) != 48 || rotated.PublicID != wh.PublicID || rotated.LastRotatedAt == nil || !rotated.LastRotatedAt.Equal(now) {
		t.Fatalf("RotateTriggerSecret = %+v, %v", rotated, err)
	}
	if kept, _, _ := rs.Trigger(ctx, wh.ID); kept.Secret != rotated.Secret {
		t.Fatalf("kept secret %q, want %q", kept.Secret, rotated.Secret)
	}
	sched, err := s.AddTrigger(ctx, 1, domain.ByMember(7), r.ID, TriggerInput{Kind: "schedule", CronExpression: "daily"}, all)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RotateTriggerSecret(ctx, 1, domain.ByMember(7), sched.ID, all); !errors.As(err, &fe) || fe.Field != "trigger.kind" {
		t.Fatalf("rotating a schedule trigger: %v", err)
	}
	if _, err := s.RotateTriggerSecret(ctx, 1, domain.ByAgent(5), wh.ID, all); !errors.Is(err, ErrNotOwnRoutine) {
		t.Fatalf("an Agent rotating another's: %v", err)
	}
	if !slices.Contains(act.actions, domain.RoutineTriggerSecretRotatedAction) {
		t.Fatalf("recorded %v", act.actions)
	}
}
