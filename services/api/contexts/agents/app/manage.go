package app

import (
	"context"
	"slices"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// MayManage reports whether the actor may manage the Agent: they hold
// hire_agents and are its Hirer, the Instance admin, or rank above the
// Hirer (the Guild Master ranks above everyone).
func (s *Service) MayManage(ctx context.Context, actor Actor, a domain.Agent) (bool, error) {
	if !slices.Contains(actor.Permissions, "hire_agents") {
		return false, nil
	}
	if actor.ID == a.HirerID || actor.InstanceAdmin {
		return true, nil
	}
	return s.guilds.RankAbove(ctx, a.GuildID, actor.ID, a.HirerID)
}

// managed is the Guild's Agent when the actor may manage it.
func (s *Service) managed(ctx context.Context, guildID uint64, actor Actor, id uint64) (domain.Agent, error) {
	a, err := s.Agent(ctx, guildID, id)
	if err != nil {
		return domain.Agent{}, err
	}
	ok, err := s.MayManage(ctx, actor, a)
	if err != nil {
		return domain.Agent{}, err
	}
	if !ok {
		return domain.Agent{}, ErrMayNotManage
	}
	return a, nil
}

// Edit changes an idle, error or paused Agent's profile, Manager and
// Heartbeat policy, or a running one's Heartbeat policy. A Patch that
// changes nothing records nothing.
func (s *Service) Edit(ctx context.Context, guildID uint64, actor Actor, id uint64, p domain.Patch) (domain.Agent, error) {
	a, err := s.managed(ctx, guildID, actor, id)
	if err != nil {
		return domain.Agent{}, err
	}
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return domain.Agent{}, err
	}
	if p.ManagerID != nil {
		if err := domain.CheckManager(all, a.ID, *p.ManagerID); err != nil {
			return domain.Agent{}, err
		}
	}
	changes, err := a.Edit(p, s.now())
	if err != nil || len(changes) == 0 {
		return a, err
	}
	if _, renamed := changes["name"]; renamed {
		for _, o := range all {
			if o.ID != a.ID && o.Status != domain.Terminated && strings.EqualFold(o.Name, a.Name) {
				return domain.Agent{}, domain.ErrNameTaken
			}
		}
	}
	save := s.agents.SaveAgent
	if p.HeartbeatOnly() {
		save = s.agents.SaveHeartbeat
	}
	if err := save(ctx, a); err != nil {
		return domain.Agent{}, err
	}
	// The Activity names Managers as {id, name}, so a later rename there
	// leaves it readable, and says only that the Capabilities changed.
	if c, ok := changes["reports_to"]; ok {
		named := func(v any) any {
			id, _ := v.(uint64)
			if i := slices.IndexFunc(all, func(o domain.Agent) bool { return o.ID == id }); id != 0 && i >= 0 {
				return map[string]any{"id": id, "name": all[i].Name}
			}
			return nil
		}
		changes["reports_to"] = domain.Change{From: named(c.From), To: named(c.To)}
	}
	s.record(ctx, domain.AgentUpdated{Agent: a, ActorID: actor.ID, Changes: changes})
	return a, nil
}

// Pause pauses an idle, running or error Agent and cancels its queued and
// running Runs.
func (s *Service) Pause(ctx context.Context, guildID uint64, actor Actor, id uint64) (domain.Agent, error) {
	a, err := s.managed(ctx, guildID, actor, id)
	if err != nil {
		return domain.Agent{}, err
	}
	if err := a.Pause(s.now()); err != nil {
		return domain.Agent{}, err
	}
	if err := s.agents.SaveAgent(ctx, a); err != nil {
		return domain.Agent{}, err
	}
	if err := s.cancelRuns(ctx, a, actor.ID); err != nil {
		return domain.Agent{}, err
	}
	s.record(ctx, domain.AgentPaused{Agent: a, ActorID: actor.ID})
	return a, nil
}

// Resume makes a paused Agent idle again.
func (s *Service) Resume(ctx context.Context, guildID uint64, actor Actor, id uint64) (domain.Agent, error) {
	a, err := s.managed(ctx, guildID, actor, id)
	if err != nil {
		return domain.Agent{}, err
	}
	if err := a.Resume(s.now()); err != nil {
		return domain.Agent{}, err
	}
	if err := s.agents.SaveAgent(ctx, a); err != nil {
		return domain.Agent{}, err
	}
	s.record(ctx, domain.AgentResumed{Agent: a, ActorID: actor.ID})
	return a, nil
}

// Terminate ends the Agent for good, by a person who may manage it.
func (s *Service) Terminate(ctx context.Context, guildID uint64, actor Actor, id uint64) (domain.Agent, error) {
	a, err := s.managed(ctx, guildID, actor, id)
	if err != nil {
		return domain.Agent{}, err
	}
	return s.terminate(ctx, a, actor.ID)
}

// terminate ends the Agent: its direct reports report to its Manager, its
// Agent membership ends, it leaves the Issues still open, its queued and
// running Runs and a hire_agent Approval still waiting are cancelled.
func (s *Service) terminate(ctx context.Context, a domain.Agent, actorID uint64) (domain.Agent, error) {
	pending := a.Status == domain.PendingApproval
	at := s.now()
	if err := a.Terminate(at); err != nil {
		return domain.Agent{}, err
	}
	if err := s.agents.SaveAgent(ctx, a); err != nil {
		return domain.Agent{}, err
	}
	all, err := s.agents.Agents(ctx, a.GuildID)
	if err != nil {
		return domain.Agent{}, err
	}
	for _, r := range all {
		if r.ManagerID == a.ID && r.Status != domain.Terminated {
			r.ManagerID, r.UpdatedAt = a.ManagerID, at
			if err := s.agents.SaveAgent(ctx, r); err != nil {
				return domain.Agent{}, err
			}
		}
	}
	if err := s.guilds.LeaveAgent(ctx, a.GuildID, a.ID); err != nil {
		return domain.Agent{}, err
	}
	if pending && a.HireApprovalID != 0 {
		if err := s.work.CancelApproval(ctx, a.GuildID, actorID, a.HireApprovalID); err != nil {
			return domain.Agent{}, err
		}
	}
	if err := s.cancelRuns(ctx, a, actorID); err != nil {
		return domain.Agent{}, err
	}
	if err := s.work.UnassignAgent(ctx, a.GuildID, a.ID, actorID); err != nil {
		return domain.Agent{}, err
	}
	s.record(ctx, domain.AgentTerminated{Agent: a, ActorID: actorID})
	return a, nil
}

// HirerLeft terminates every Agent the Member hired in the Guild, once
// they have left it or were removed by actorID, so no Agent is left
// without a Hirer whose desktop runs it.
func (s *Service) HirerLeft(ctx context.Context, guildID, memberID, actorID uint64) error {
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return err
	}
	for _, a := range all {
		if a.HirerID != memberID || a.Status == domain.Terminated {
			continue
		}
		// Reload: terminating an earlier one may have moved this one up.
		a, err := s.Agent(ctx, guildID, a.ID)
		if err != nil {
			return err
		}
		if _, err := s.terminate(ctx, a, actorID); err != nil {
			return err
		}
	}
	return nil
}

// AddRole gives an idle, error or paused Agent a Role; one it holds already
// changes and records nothing.
func (s *Service) AddRole(ctx context.Context, guildID uint64, actor Actor, id, roleID uint64) (domain.Agent, error) {
	return s.reRole(ctx, guildID, actor, id, roleID, true)
}

// RemoveRole takes a Role from an idle, error or paused Agent; one it does not
// hold changes and records nothing.
func (s *Service) RemoveRole(ctx context.Context, guildID uint64, actor Actor, id, roleID uint64) (domain.Agent, error) {
	return s.reRole(ctx, guildID, actor, id, roleID, false)
}

func (s *Service) reRole(ctx context.Context, guildID uint64, actor Actor, id, roleID uint64, add bool) (domain.Agent, error) {
	a, err := s.managed(ctx, guildID, actor, id)
	if err != nil {
		return domain.Agent{}, err
	}
	if err := a.Rolable(); err != nil {
		return domain.Agent{}, err
	}
	before, err := s.guilds.AgentRoles(ctx, guildID, a.ID)
	if err != nil {
		return domain.Agent{}, err
	}
	held := slices.ContainsFunc(before, func(r Role) bool { return r.ID == roleID })
	if add {
		err = s.guilds.AssignAgentRole(ctx, guildID, actor, a.ID, roleID)
	} else {
		err = s.guilds.RemoveAgentRole(ctx, guildID, actor, a.ID, roleID)
	}
	if err != nil || held == add {
		return a, err
	}
	names, err := s.guilds.RoleNames(ctx, guildID, []uint64{roleID})
	if err != nil {
		return domain.Agent{}, err
	}
	var role string
	if len(names) > 0 {
		role = names[0]
	}
	if add {
		s.record(ctx, domain.AgentRoleAdded{Agent: a, ActorID: actor.ID, Role: role})
	} else {
		s.record(ctx, domain.AgentRoleRemoved{Agent: a, ActorID: actor.ID, Role: role})
	}
	return a, nil
}
