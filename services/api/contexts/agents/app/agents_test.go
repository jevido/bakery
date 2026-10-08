package app

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type fakeAgents struct {
	rows map[uint64]domain.Agent
	next uint64
}

func (f *fakeAgents) Agents(_ context.Context, guildID uint64) ([]domain.Agent, error) {
	var out []domain.Agent
	for id := uint64(1); id <= f.next; id++ {
		if a, ok := f.rows[id]; ok && a.GuildID == guildID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeAgents) Agent(_ context.Context, id uint64) (domain.Agent, bool, error) {
	a, ok := f.rows[id]
	return a, ok, nil
}
func (f *fakeAgents) CreateAgent(_ context.Context, a domain.Agent) (domain.Agent, error) {
	f.next++
	a.ID = f.next
	f.rows[a.ID] = a
	return a, nil
}
func (f *fakeAgents) SaveAgent(_ context.Context, a domain.Agent) error { f.rows[a.ID] = a; return nil }
func (f *fakeAgents) SaveHeartbeat(_ context.Context, a domain.Agent) error {
	r := f.rows[a.ID]
	r.Heartbeat, r.UpdatedAt = a.Heartbeat, a.UpdatedAt
	f.rows[a.ID] = r
	return nil
}
func (f *fakeAgents) DeleteAgent(_ context.Context, id uint64) error { delete(f.rows, id); return nil }

type fakeGuilds struct {
	joined map[uint64][]uint64
	refuse error
	leaves int
	// rank is each Member's rank; higher ranks above.
	rank map[uint64]int
}

func (f *fakeGuilds) JoinAgent(_ context.Context, _, _, agentID uint64, _ []string, roleIDs []uint64) error {
	if f.refuse != nil {
		return f.refuse
	}
	f.joined[agentID] = roleIDs
	return nil
}
func (f *fakeGuilds) LeaveAgent(_ context.Context, _, agentID uint64) error {
	f.leaves++
	delete(f.joined, agentID)
	return nil
}
func (f *fakeGuilds) AgentRoles(_ context.Context, _, agentID uint64) ([]Role, error) {
	var out []Role
	for _, id := range f.joined[agentID] {
		out = append(out, Role{ID: id})
	}
	return out, nil
}
func (f *fakeGuilds) AssignAgentRole(_ context.Context, _ uint64, _ Actor, agentID, roleID uint64) error {
	if f.refuse != nil {
		return f.refuse
	}
	if !slices.Contains(f.joined[agentID], roleID) {
		f.joined[agentID] = append(f.joined[agentID], roleID)
	}
	return nil
}
func (f *fakeGuilds) RemoveAgentRole(_ context.Context, _ uint64, _ Actor, agentID, roleID uint64) error {
	f.joined[agentID] = slices.DeleteFunc(f.joined[agentID], func(id uint64) bool { return id == roleID })
	return nil
}
func (f *fakeGuilds) RankAbove(_ context.Context, _, actorID, memberID uint64) (bool, error) {
	return f.rank[actorID] > f.rank[memberID], nil
}
func (f *fakeGuilds) GuildName(_ context.Context, guildID uint64) (string, error) {
	return "Guild " + strconv.FormatUint(guildID, 10), nil
}

func (f *fakeGuilds) RoleNames(_ context.Context, _ uint64, ids []uint64) ([]string, error) {
	out := make([]string, len(ids))
	for i := range ids {
		out[i] = "Deployer"
	}
	return out, nil
}

type fakeWork struct {
	fail      error
	requests  []HireRequest
	actions   []string
	cancelled []uint64
	last      Activity
	// unassigned are the Agents taken off Issues.
	unassigned []uint64
	issues     map[uint64]IssueBrief
}

func (f *fakeWork) IssueForRun(_ context.Context, _, issueID uint64) (IssueBrief, bool, error) {
	i, ok := f.issues[issueID]
	return i, ok, nil
}

func (f *fakeWork) UnassignAgent(_ context.Context, _, agentID, _ uint64) error {
	f.unassigned = append(f.unassigned, agentID)
	return nil
}

func (f *fakeWork) CancelApproval(_ context.Context, _, _, approvalID uint64) error {
	f.cancelled = append(f.cancelled, approvalID)
	return nil
}

func (f *fakeWork) RequestHireApproval(_ context.Context, _, _ uint64, r HireRequest) (uint64, error) {
	if f.fail != nil {
		return 0, f.fail
	}
	f.requests = append(f.requests, r)
	return 90 + uint64(len(f.requests)), nil
}
func (f *fakeWork) RecordActivity(_ context.Context, e Activity) error {
	f.actions = append(f.actions, e.Action)
	f.last = e
	return nil
}

func newTest() (*Service, *fakeAgents, *fakeGuilds, *fakeWork) {
	a, g, w := &fakeAgents{rows: map[uint64]domain.Agent{}}, &fakeGuilds{joined: map[uint64][]uint64{}, rank: map[uint64]int{}}, &fakeWork{}
	return NewService(a, &fakeRuns{rows: map[uint64]domain.Run{}, events: map[uint64][]domain.RunEvent{}, agents: a}, g, w), a, g, w
}

func TestHireAndDecide(t *testing.T) {
	ctx := context.Background()
	s, store, g, w := newTest()
	ada, approval, err := s.Hire(ctx, 1, 7, nil, HireInput{Profile: domain.Profile{Name: "Ada", Job: "cto"}, RoleIDs: []uint64{4}})
	if err != nil || ada.Status != domain.PendingApproval || approval != 91 || store.rows[ada.ID].HireApprovalID != 91 {
		t.Fatalf("hire: %+v %d %v", ada, approval, err)
	}
	if len(g.joined[ada.ID]) != 1 || len(w.actions) != 1 || w.actions[0] != "agent.hired" {
		t.Fatalf("joined %v, actions %v", g.joined, w.actions)
	}
	bob, _, err := s.Hire(ctx, 1, 7, nil, HireInput{Profile: domain.Profile{Name: "Bob"}, ManagerID: ada.ID})
	if err != nil || w.requests[1].ManagerName != "Ada" {
		t.Fatalf("bob: %v %+v", err, w.requests)
	}
	if _, _, err := s.Hire(ctx, 1, 7, nil, HireInput{Profile: domain.Profile{Name: "ADA"}}); !errors.Is(err, domain.ErrNameTaken) {
		t.Fatalf("duplicate name: %v", err)
	}
	if err := s.Decided(ctx, Decision{GuildID: 1, AgentID: ada.ID, Approved: true}); err != nil || store.rows[ada.ID].Status != domain.Idle {
		t.Fatalf("approved: %v %s", err, store.rows[ada.ID].Status)
	}
	if err := s.Decided(ctx, Decision{GuildID: 1, AgentID: ada.ID, Approved: true}); err != nil {
		t.Fatalf("approved again: %v", err)
	}
	for range 2 {
		if err := s.Decided(ctx, Decision{GuildID: 1, AgentID: bob.ID, DeciderID: 7}); err != nil {
			t.Fatalf("rejected: %v", err)
		}
	}
	if store.rows[bob.ID].Status != domain.Terminated || g.joined[bob.ID] != nil {
		t.Fatalf("bob after reject: %+v %v", store.rows[bob.ID], g.joined)
	}
	if n := len(w.actions); n != 3 || w.actions[2] != "agent.terminated" {
		t.Fatalf("actions %v", w.actions)
	}
	if org, _ := s.Org(ctx, 1); len(org) != 1 || org[0].Agent.ID != ada.ID || len(org[0].Reports) != 0 {
		t.Fatalf("org %+v", org)
	}
	if as, _ := s.Agents(ctx, 1, "terminated"); len(as) != 1 || as[0].ID != bob.ID {
		t.Fatalf("terminated: %+v", as)
	}
	if _, err := s.Agents(ctx, 1, "running"); !errors.Is(err, ErrUnknownFilter) {
		t.Fatalf("filter: %v", err)
	}
	if _, err := s.Agent(ctx, 2, ada.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other guild: %v", err)
	}
}

func TestHireLeavesNothingOnFailure(t *testing.T) {
	ctx := context.Background()
	s, store, g, w := newTest()
	refused := &domain.FieldError{Field: "role_ids", Message: "no"}
	g.refuse = refused
	if _, _, err := s.Hire(ctx, 1, 7, nil, HireInput{Profile: domain.Profile{Name: "Ada"}}); !errors.Is(err, refused) {
		t.Fatalf("refused role: %v", err)
	}
	g.refuse, w.fail = nil, errors.New("down")
	if _, _, err := s.Hire(ctx, 1, 7, nil, HireInput{Profile: domain.Profile{Name: "Ada"}}); !errors.Is(err, w.fail) {
		t.Fatalf("approval failed: %v", err)
	}
	if len(store.rows) != 0 || len(g.joined) != 0 || g.leaves != 1 || len(w.actions) != 0 {
		t.Fatalf("left behind: %v %v %d %v", store.rows, g.joined, g.leaves, w.actions)
	}
}
