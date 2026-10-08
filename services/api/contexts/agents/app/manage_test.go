package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

var hirer = []string{"view_resources", "hire_agents"}

// hired hires an Agent by Member 7 and approves it.
func hired(t *testing.T, s *Service, name string, manager uint64) domain.Agent {
	t.Helper()
	ctx := context.Background()
	a, _, err := s.Hire(ctx, 1, 7, hirer, HireInput{Profile: domain.Profile{Name: name}, ManagerID: manager})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Decided(ctx, Decision{GuildID: 1, AgentID: a.ID, DeciderID: 1, Approved: true}); err != nil {
		t.Fatal(err)
	}
	a, _ = s.Agent(ctx, 1, a.ID)
	return a
}

func TestMayManage(t *testing.T) {
	ctx := context.Background()
	s, _, g, _ := newTest()
	ada := hired(t, s, "Ada", 0)
	// 1 is the Guild Master, 8 ranks above the Hirer 7, 9 below.
	g.rank = map[uint64]int{1: 100, 8: 10, 7: 5, 9: 1}
	for _, c := range []struct {
		actor Actor
		want  bool
	}{
		{Actor{ID: 7, Permissions: hirer}, true},
		{Actor{ID: 8, Permissions: hirer}, true},
		{Actor{ID: 9, Permissions: hirer}, false},
		{Actor{ID: 1, Permissions: hirer}, true},
		{Actor{ID: 8, Permissions: []string{"view_resources"}}, false},
		{Actor{ID: 9, Permissions: hirer, InstanceAdmin: true}, true},
	} {
		if got, err := s.MayManage(ctx, c.actor, ada); err != nil || got != c.want {
			t.Errorf("%+v: %v %v, want %v", c.actor, got, err, c.want)
		}
	}
	if _, err := s.Pause(ctx, 1, Actor{ID: 9, Permissions: hirer}, ada.ID); !errors.Is(err, ErrMayNotManage) {
		t.Errorf("pause by someone below: %v", err)
	}
}

func TestEditPauseResume(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	me := Actor{ID: 7, Permissions: hirer}
	ada := hired(t, s, "Ada", 0)
	bob := hired(t, s, "Bob", ada.ID)
	if _, err := s.Edit(ctx, 1, me, ada.ID, domain.Patch{ManagerID: &bob.ID}); !errors.Is(err, domain.ErrManagerCycle) {
		t.Errorf("cycle: %v", err)
	}
	if _, err := s.Edit(ctx, 1, me, bob.ID, domain.Patch{Name: ptr("ADA")}); !errors.Is(err, domain.ErrNameTaken) {
		t.Errorf("name taken: %v", err)
	}
	if _, err := s.Edit(ctx, 1, me, bob.ID, domain.Patch{ManagerID: ptr[uint64](0), Capabilities: ptr("Go")}); err != nil {
		t.Fatal(err)
	}
	ch := w.last.Details["changes"].(map[string]any)
	if rt := ch["reports_to"].(map[string]any); rt["from"].(map[string]any)["name"] != "Ada" || rt["to"] != nil || ch["capabilities"] != true {
		t.Errorf("changes %v", ch)
	}
	w.actions = nil
	title := "Head"
	if a, err := s.Edit(ctx, 1, me, ada.ID, domain.Patch{Title: &title}); err != nil || a.Title != "Head" {
		t.Fatalf("edit: %+v %v", a, err)
	}
	if _, err := s.Edit(ctx, 1, me, ada.ID, domain.Patch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if a, err := s.Pause(ctx, 1, me, ada.ID); err != nil || a.Status != domain.Paused || a.PausedAt == nil {
		t.Fatalf("pause: %+v %v", a, err)
	}
	var se *domain.StatusError
	if _, err := s.Pause(ctx, 1, me, ada.ID); !errors.As(err, &se) {
		t.Errorf("pause again: %v", err)
	}
	if a, err := s.Resume(ctx, 1, me, ada.ID); err != nil || a.Status != domain.Idle || a.PausedAt != nil {
		t.Fatalf("resume: %+v %v", a, err)
	}
	if want := []string{"agent.updated", "agent.paused", "agent.resumed"}; !slices.Equal(w.actions, want) {
		t.Errorf("actions %v, want %v", w.actions, want)
	}
}

func ptr[T any](v T) *T { return &v }

func TestTerminateMovesReportsUp(t *testing.T) {
	ctx := context.Background()
	s, store, g, w := newTest()
	me := Actor{ID: 7, Permissions: hirer}
	ceo := hired(t, s, "Ceo", 0)
	ada := hired(t, s, "Ada", ceo.ID)
	bob := hired(t, s, "Bob", ada.ID)
	a, err := s.Terminate(ctx, 1, me, ada.ID)
	if err != nil || a.Status != domain.Terminated {
		t.Fatalf("terminate: %+v %v", a, err)
	}
	if store.rows[bob.ID].ManagerID != ceo.ID {
		t.Errorf("bob reports to %d, want %d", store.rows[bob.ID].ManagerID, ceo.ID)
	}
	if _, ok := g.joined[ada.ID]; ok {
		t.Error("ada kept her agent membership")
	}
	if _, err := s.Resume(ctx, 1, me, ada.ID); err == nil {
		t.Error("a terminated agent was resumed")
	}
	if !slices.Equal(w.unassigned, []uint64{ada.ID}) {
		t.Errorf("unassigned %v, want ada", w.unassigned)
	}
	if len(w.cancelled) != 0 {
		t.Errorf("an approved hire was cancelled: %v", w.cancelled)
	}
	cy, approval, _ := s.Hire(ctx, 1, 7, hirer, HireInput{Profile: domain.Profile{Name: "Cy"}})
	if _, err := s.Terminate(ctx, 1, me, cy.ID); err != nil || !slices.Equal(w.cancelled, []uint64{approval}) {
		t.Errorf("terminating a pending hire: %v, cancelled %v", err, w.cancelled)
	}
}

func TestRolesAndHirerLeaving(t *testing.T) {
	ctx := context.Background()
	s, store, g, w := newTest()
	me := Actor{ID: 7, Permissions: hirer}
	ada := hired(t, s, "Ada", 0)
	bob := hired(t, s, "Bob", ada.ID)
	w.actions = nil
	for range 2 {
		if _, err := s.AddRole(ctx, 1, me, ada.ID, 4); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RemoveRole(ctx, 1, me, ada.ID, 4); err != nil {
		t.Fatal(err)
	}
	if want := []string{"agent.role_added", "agent.role_removed"}; !slices.Equal(w.actions, want) {
		t.Errorf("actions %v, want %v", w.actions, want)
	}
	g.refuse = &domain.FieldError{Field: "role_id", Message: "above the hirer"}
	if _, err := s.AddRole(ctx, 1, me, ada.ID, 2); !errors.Is(err, g.refuse) {
		t.Errorf("refused role: %v", err)
	}
	g.refuse = nil
	cy, _, _ := s.Hire(ctx, 1, 7, hirer, HireInput{Profile: domain.Profile{Name: "Cy"}})
	if _, err := s.AddRole(ctx, 1, me, cy.ID, 4); err == nil {
		t.Error("a pending hire was given a role")
	}
	other, _, _ := s.Hire(ctx, 1, 8, hirer, HireInput{Profile: domain.Profile{Name: "Other"}})
	if err := s.HirerLeft(ctx, 1, 7, 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint64{ada.ID, bob.ID, cy.ID} {
		if store.rows[id].Status != domain.Terminated {
			t.Errorf("agent %d is %s", id, store.rows[id].Status)
		}
	}
	if store.rows[other.ID].Status != domain.PendingApproval {
		t.Error("another hirer's agent was terminated")
	}
}

func TestEditHeartbeat(t *testing.T) {
	ctx := context.Background()
	s, _, g, w := newTest()
	ada := hired(t, s, "Ada", 0)
	g.rank = map[uint64]int{7: 5, 9: 1}
	sec := 120
	p := domain.Patch{Heartbeat: &domain.HeartbeatPatch{Enabled: ptr(true), IntervalSec: &sec}}
	if _, err := s.Edit(ctx, 1, Actor{ID: 9, Permissions: hirer}, ada.ID, p); !errors.Is(err, ErrMayNotManage) {
		t.Errorf("edit by someone below: %v", err)
	}
	a, err := s.Edit(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, p)
	if err != nil || !a.Heartbeat.Enabled || a.Heartbeat.IntervalSec != 120 || !a.Heartbeat.WakeOnDemand {
		t.Fatalf("edit: %+v %v", a.Heartbeat, err)
	}
	ch := w.last.Details["changes"].(map[string]any)
	if iv := ch["heartbeat.interval_sec"].(map[string]any); iv["from"] != 300 || iv["to"] != 120 || ch["heartbeat.enabled"] == nil {
		t.Errorf("changes %v", ch)
	}
	if _, err := s.Terminate(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID); err != nil {
		t.Fatal(err)
	}
	var se *domain.StatusError
	if _, err := s.Edit(ctx, 1, Actor{ID: 7, Permissions: hirer}, ada.ID, p); !errors.As(err, &se) {
		t.Errorf("edit terminated: %v", err)
	}
}
