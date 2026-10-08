// Package app holds the agents use cases: hire an Agent, follow its
// hire_agent Approval, and read the Guild's Agents and Org chart.
package app

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// ErrNotFound is an Agent that does not exist in the Guild.
var ErrNotFound = errors.New("not found")

// Agents keeps Agents.
type Agents interface {
	// Agents lists every Agent of the Guild, terminated ones included.
	Agents(ctx context.Context, guildID uint64) ([]domain.Agent, error)
	Agent(ctx context.Context, id uint64) (domain.Agent, bool, error)
	// CreateAgent stores a new Agent; domain.ErrNameTaken when another one
	// of the Guild that is not terminated has its name.
	CreateAgent(ctx context.Context, a domain.Agent) (domain.Agent, error)
	SaveAgent(ctx context.Context, a domain.Agent) error
	// DeleteAgent removes an Agent whose hire did not go through.
	DeleteAgent(ctx context.Context, id uint64) error
}

// Role is a Guild's Role an Agent holds.
type Role struct {
	ID       uint64
	Name     string
	Color    string
	Position int
}

// Guilds is what agents asks guilds about Agent memberships. A Role
// refused to the Agent is a *domain.FieldError on role_ids.
type Guilds interface {
	JoinAgent(ctx context.Context, guildID, hirerID, agentID uint64, hirerPerms []string, roleIDs []uint64) error
	LeaveAgent(ctx context.Context, guildID, agentID uint64) error
	AgentRoles(ctx context.Context, guildID, agentID uint64) ([]Role, error)
	RoleNames(ctx context.Context, guildID uint64, ids []uint64) ([]string, error)
}

// HireRequest is the payload of a hire_agent Approval.
type HireRequest struct {
	Agent       domain.Agent
	ManagerName string
	Roles       []string
}

// Activity is an Activity event about an Agent.
type Activity struct {
	GuildID, ActorID, AgentID uint64
	Action                    string
	AgentName                 string
	Details                   map[string]any
}

// Work is what agents asks work: a hire_agent Approval for a hired Agent,
// and an Activity event for each of its changes.
type Work interface {
	RequestHireApproval(ctx context.Context, guildID, hirerID uint64, r HireRequest) (uint64, error)
	RecordActivity(ctx context.Context, e Activity) error
}

type Service struct {
	agents Agents
	guilds Guilds
	work   Work
	now    func() time.Time
	// Logf logs what a request cannot report, such as an Activity event
	// that was not recorded.
	Logf func(format string, args ...any)
}

func NewService(agents Agents, guilds Guilds, work Work) *Service {
	return &Service{agents: agents, guilds: guilds, work: work, now: time.Now, Logf: log.Printf}
}

// HireInput is a new Agent as typed: ManagerID 0 reports to no one,
// RoleIDs are the Roles of its Agent membership.
type HireInput struct {
	domain.Profile
	ManagerID uint64
	RoleIDs   []uint64
}

// Hire makes the Agent pending_approval with its Agent membership and
// Roles, and asks the Board for its hire_agent Approval. When either step
// fails, nothing of the Agent is left.
func (s *Service) Hire(ctx context.Context, guildID, hirerID uint64, hirerPerms []string, in HireInput) (domain.Agent, uint64, error) {
	a, err := domain.Hire(guildID, hirerID, in.Profile, in.ManagerID, s.now())
	if err != nil {
		return domain.Agent{}, 0, err
	}
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return domain.Agent{}, 0, err
	}
	if err := domain.CheckManager(all, 0, in.ManagerID); err != nil {
		return domain.Agent{}, 0, err
	}
	for _, o := range all {
		if o.Status != domain.Terminated && strings.EqualFold(o.Name, a.Name) {
			return domain.Agent{}, 0, domain.ErrNameTaken
		}
	}
	if a, err = s.agents.CreateAgent(ctx, a); err != nil {
		return domain.Agent{}, 0, err
	}
	undo := func(cause error, joined bool) (domain.Agent, uint64, error) {
		if joined {
			if err := s.guilds.LeaveAgent(ctx, guildID, a.ID); err != nil {
				s.Logf("agents: undoing the agent membership of agent %d: %v", a.ID, err)
			}
		}
		if err := s.agents.DeleteAgent(ctx, a.ID); err != nil {
			s.Logf("agents: undoing agent %d: %v", a.ID, err)
		}
		return domain.Agent{}, 0, cause
	}
	if err := s.guilds.JoinAgent(ctx, guildID, hirerID, a.ID, hirerPerms, in.RoleIDs); err != nil {
		return undo(err, false)
	}
	req := HireRequest{Agent: a}
	if i := slices.IndexFunc(all, func(o domain.Agent) bool { return o.ID == a.ManagerID }); i >= 0 {
		req.ManagerName = all[i].Name
	}
	if req.Roles, err = s.guilds.RoleNames(ctx, guildID, in.RoleIDs); err != nil {
		return undo(err, true)
	}
	approvalID, err := s.work.RequestHireApproval(ctx, guildID, hirerID, req)
	if err != nil {
		return undo(err, true)
	}
	a.HireApprovalID = approvalID
	if err := s.agents.SaveAgent(ctx, a); err != nil {
		return domain.Agent{}, 0, err
	}
	s.record(ctx, domain.AgentHired{Agent: a, ActorID: hirerID})
	return a, approvalID, nil
}

// record adds a domain event to work's Activity; a failure is logged, as
// the change it tells of is stored already.
func (s *Service) record(ctx context.Context, e any) {
	var act Activity
	switch e := e.(type) {
	case domain.AgentHired:
		act = activityOf(e.Agent, e.ActorID, "agent.hired")
		act.Details["job"], act.Details["approval_id"] = string(e.Agent.Job), e.Agent.HireApprovalID
	case domain.AgentTerminated:
		act = activityOf(e.Agent, e.ActorID, "agent.terminated")
	default:
		return
	}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording %s of agent %d: %v", act.Action, act.AgentID, err)
	}
}

func activityOf(a domain.Agent, actorID uint64, action string) Activity {
	return Activity{GuildID: a.GuildID, ActorID: actorID, AgentID: a.ID, Action: action, AgentName: a.Name, Details: map[string]any{"name": a.Name}}
}

// Decision is the Board's Decision on an Agent's hire_agent Approval.
type Decision struct {
	GuildID, AgentID, DeciderID uint64
	Approved                    bool
}

// Decided follows the Decision: approved makes the Agent idle, rejected
// terminates it and ends its Agent membership. The same Decision again
// changes nothing but heals an Agent membership left behind.
func (s *Service) Decided(ctx context.Context, d Decision) error {
	a, err := s.Agent(ctx, d.GuildID, d.AgentID)
	if err != nil {
		return err
	}
	at := s.now()
	if d.Approved {
		changed, err := a.Approve(at)
		if err != nil || !changed {
			return err
		}
		return s.agents.SaveAgent(ctx, a)
	}
	changed, err := a.Reject(at)
	if err != nil {
		return err
	}
	if changed {
		if err := s.agents.SaveAgent(ctx, a); err != nil {
			return err
		}
	}
	if err := s.guilds.LeaveAgent(ctx, d.GuildID, a.ID); err != nil {
		return err
	}
	if changed {
		s.record(ctx, domain.AgentTerminated{Agent: a, ActorID: d.DeciderID})
	}
	return nil
}

// Filters of the Agents list: all is every Agent but the terminated ones,
// as Paperclip's All tab; active is idle.
var filters = map[string][]domain.Status{
	"all":        {domain.PendingApproval, domain.Idle, domain.Paused},
	"active":     {domain.Idle},
	"paused":     {domain.Paused},
	"pending":    {domain.PendingApproval},
	"terminated": {domain.Terminated},
}

// ErrUnknownFilter is a status filter that is none of all, active,
// paused, pending or terminated.
var ErrUnknownFilter error = &domain.FieldError{Field: "status", Message: "must be one of all, active, paused, pending, terminated"}

// Agents lists the Guild's Agents in the filter ("" is all), by name.
func (s *Service) Agents(ctx context.Context, guildID uint64, filter string) ([]domain.Agent, error) {
	if filter == "" {
		filter = "all"
	}
	keep, ok := filters[filter]
	if !ok {
		return nil, ErrUnknownFilter
	}
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := slices.DeleteFunc(all, func(a domain.Agent) bool { return !slices.Contains(keep, a.Status) })
	slices.SortStableFunc(out, func(x, y domain.Agent) int { return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)) })
	return out, nil
}

// Agent is the Guild's Agent; ErrNotFound when it is another Guild's.
func (s *Service) Agent(ctx context.Context, guildID, id uint64) (domain.Agent, error) {
	a, ok, err := s.agents.Agent(ctx, id)
	if err != nil {
		return domain.Agent{}, err
	}
	if !ok || a.GuildID != guildID {
		return domain.Agent{}, ErrNotFound
	}
	return a, nil
}

// AgentInGuild reports whether the Agent belongs to the Guild.
func (s *Service) AgentInGuild(ctx context.Context, id, guildID uint64) (bool, error) {
	a, ok, err := s.agents.Agent(ctx, id)
	return ok && a.GuildID == guildID, err
}

// Org is the Guild's Org chart.
func (s *Service) Org(ctx context.Context, guildID uint64) ([]domain.Node, error) {
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return nil, err
	}
	return domain.Org(all), nil
}

// Names names the Guild's Agents among ids that are not terminated.
func (s *Service) Names(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := map[uint64]string{}
	for _, a := range all {
		if a.Status != domain.Terminated && slices.Contains(ids, a.ID) {
			out[a.ID] = a.Name
		}
	}
	return out, nil
}

// HasAgents reports whether the Guild has an Agent that is not
// terminated, which keeps it from being deleted.
func (s *Service) HasAgents(ctx context.Context, guildID uint64) (bool, error) {
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(all, func(a domain.Agent) bool { return a.Status != domain.Terminated }), nil
}

// Roles lists the Roles the Agent holds, top first.
func (s *Service) Roles(ctx context.Context, a domain.Agent) ([]Role, error) {
	if a.Status == domain.Terminated {
		return []Role{}, nil
	}
	return s.guilds.AgentRoles(ctx, a.GuildID, a.ID)
}
