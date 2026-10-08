package app

import (
	"context"
	"errors"
	"slices"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentMemberAlready = errors.New("the agent has a membership already")
)

// JoinAgent gives an Agent its Agent membership in the Guild, hired by
// hirerID, a Member there, holding roleIDs: each a Role the Hirer may
// assign, below their own highest, with Permissions they hold.
func (s *Service) JoinAgent(ctx context.Context, guildID, hirerID, agentID uint64, hirerPerms domain.Permissions, roleIDs []uint64) (domain.Membership, error) {
	c, err := s.changeRoles(ctx, guildID, hirerID, hirerPerms, func(h Hierarchy, a domain.Actor) (Change, error) {
		if _, ok := membershipOf(h.Memberships, hirerID); !ok {
			return Change{}, ErrMembershipNotFound
		}
		if _, ok := agentOf(h.Memberships, agentID); ok {
			return Change{}, ErrAgentMemberAlready
		}
		held := []uint64{}
		for _, id := range roleIDs {
			r, ok := findRole(h.Roles, id)
			if !ok {
				return Change{}, ErrRoleNotFound
			}
			if err := domain.CanAssignToAgent(a.Rank, int(a.Rank), r); err != nil {
				return Change{}, err
			}
			if err := domain.CanGrant(a.Permissions, 0, r.Permissions); err != nil {
				return Change{}, err
			}
			if !slices.Contains(held, id) {
				held = append(held, id)
			}
		}
		return Change{Agents: []domain.Membership{{GuildID: guildID, AgentID: agentID, HirerID: hirerID, RoleIDs: held}}}, nil
	})
	if err != nil {
		return domain.Membership{}, err
	}
	m, _ := agentOf(c.Agents, agentID)
	return m, nil
}

// AssignAgentRole gives an Agent of the Guild a Role below the actor's
// highest and below its Hirer's, with Permissions the actor holds. The
// agents context has already checked that the actor may manage this
// Agent, so manage_roles is not needed.
func (s *Service) AssignAgentRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, agentID, roleID uint64) (domain.Membership, error) {
	return s.reRoleAgent(ctx, guildID, actorID, perms, agentID, roleID, func(h Hierarchy, a domain.Actor, agent domain.Membership, r domain.Role) ([]uint64, error) {
		hirer, err := s.hirerRank(ctx, h, agent)
		if err != nil {
			return nil, err
		}
		if err := domain.CanAssignToAgent(a.Rank, int(hirer), r); err != nil {
			return nil, err
		}
		// As at the hire: the actor may lack manage_roles, so a Role
		// below them must not hand the Agent a Permission they lack.
		if err := domain.CanGrant(a.Permissions, 0, r.Permissions); err != nil {
			return nil, err
		}
		if slices.Contains(agent.RoleIDs, r.ID) {
			return agent.RoleIDs, nil
		}
		return append(slices.Clone(agent.RoleIDs), r.ID), nil
	})
}

// RemoveAgentRole takes a Role below the actor's highest from an Agent of
// the Guild, as AssignAgentRole.
func (s *Service) RemoveAgentRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, agentID, roleID uint64) (domain.Membership, error) {
	return s.reRoleAgent(ctx, guildID, actorID, perms, agentID, roleID, func(_ Hierarchy, a domain.Actor, agent domain.Membership, r domain.Role) ([]uint64, error) {
		if err := domain.CanAssign(a.Rank, r); err != nil {
			return nil, err
		}
		return slices.DeleteFunc(slices.Clone(agent.RoleIDs), func(id uint64) bool { return id == r.ID }), nil
	})
}

func (s *Service) reRoleAgent(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, agentID, roleID uint64, to func(Hierarchy, domain.Actor, domain.Membership, domain.Role) ([]uint64, error)) (domain.Membership, error) {
	c, err := s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		agent, ok := agentOf(h.Memberships, agentID)
		if !ok {
			return Change{}, ErrAgentNotFound
		}
		r, ok := findRole(h.Roles, roleID)
		if !ok {
			return Change{}, ErrRoleNotFound
		}
		held, err := to(h, a, agent, r)
		if err != nil {
			return Change{}, err
		}
		agent.RoleIDs = held
		return Change{Agents: []domain.Membership{agent}}, nil
	})
	if err != nil {
		return domain.Membership{}, err
	}
	m, _ := agentOf(c.Agents, agentID)
	return m, nil
}

// LeaveAgent ends an Agent's Agent membership in the Guild, with the Roles
// it held; nothing when it has none.
func (s *Service) LeaveAgent(ctx context.Context, guildID, agentID uint64) error {
	_, err := s.change(ctx, guildID, func(h Hierarchy) (Change, error) {
		m, ok := agentOf(h.Memberships, agentID)
		if !ok {
			return Change{}, nil
		}
		return Change{RemovedMembership: m.ID}, nil
	})
	return err
}

// AgentPermissions is what the Agent's Roles in the Guild allow, the Base
// role's included, as for any Membership.
func (s *Service) AgentPermissions(ctx context.Context, guildID, agentID uint64) (domain.Permissions, error) {
	p, err := s.agentPlace(ctx, domain.Guild{ID: guildID}, agentID)
	return p.Permissions, err
}

// AgentInProject is what the Agent may do in one of the Guild's Projects:
// its Permissions with the Project's Overrides of the Roles it holds
// applied. A person's own override never names an Agent.
func (s *Service) AgentInProject(ctx context.Context, guildID, agentID, projectID uint64) (domain.Permissions, error) {
	p, err := s.agentPlace(ctx, domain.Guild{ID: guildID}, agentID)
	if err != nil {
		return 0, err
	}
	return s.InProject(ctx, p, projectID)
}

// AgentPlace is where an Agent principal's request acts: the Guild of its
// Run with its Agent membership's Permissions there, the Base role's
// included, and the Roles it holds for InProject. MemberID stays 0, so no
// person's own override applies. ErrGuildNotFound or ErrAgentNotFound when
// either is gone.
func (s *Service) AgentPlace(ctx context.Context, guildID, agentID uint64) (Place, error) {
	g, err := s.Guild(ctx, guildID)
	if err != nil {
		return Place{}, err
	}
	return s.agentPlace(ctx, g, agentID)
}

func (s *Service) agentPlace(ctx context.Context, g domain.Guild, agentID uint64) (Place, error) {
	m, ok, err := s.memberships.OfAgent(ctx, g.ID, agentID)
	if err != nil {
		return Place{}, err
	}
	if !ok {
		return Place{}, ErrAgentNotFound
	}
	roles, err := s.roles.ForGuild(ctx, g.ID)
	if err != nil {
		return Place{}, err
	}
	return held(Place{Guild: g, Permissions: domain.PermissionsOf(roles, m)}, roles, m), nil
}

// AgentRoles lists the Roles the Agent holds in the Guild besides the Base
// role, top first.
func (s *Service) AgentRoles(ctx context.Context, guildID, agentID uint64) ([]domain.Role, error) {
	m, ok, err := s.memberships.OfAgent(ctx, guildID, agentID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAgentNotFound
	}
	return s.RolesAmong(ctx, guildID, m.RoleIDs)
}

// RolesAmong lists the Guild's Roles among ids, top first, leaving out
// ids that are not the Guild's and the Base role.
func (s *Service) RolesAmong(ctx context.Context, guildID uint64, ids []uint64) ([]domain.Role, error) {
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := []domain.Role{}
	for i := len(roles) - 1; i >= 0; i-- {
		if r := roles[i]; !r.Base && slices.Contains(ids, r.ID) {
			out = append(out, r)
		}
	}
	return out, nil
}

// RankAbove reports whether actorID ranks above memberID in the Guild: the
// Guild Master above everyone, the Instance admin above everyone else, and
// otherwise whoever's highest Role is higher.
func (s *Service) RankAbove(ctx context.Context, guildID, actorID, memberID uint64) (bool, error) {
	g, err := s.Guild(ctx, guildID)
	if err != nil {
		return false, err
	}
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return false, err
	}
	rank := func(id uint64) (domain.Rank, error) {
		admin, err := s.members.IsInstanceAdmin(ctx, id)
		if err != nil {
			return 0, err
		}
		m, _, err := s.memberships.Of(ctx, guildID, id)
		return domain.RankOf(g, roles, m, id, admin), err
	}
	a, err := rank(actorID)
	if err != nil {
		return false, err
	}
	b, err := rank(memberID)
	return a > b, err
}

// OnMemberLeaving registers f, called once a Member has left the Guild or
// been removed from it by actorID (the Member themself when they left);
// the agents context terminates the Agents they hired there.
func (s *Service) OnMemberLeaving(f func(ctx context.Context, guildID, memberID, actorID uint64) error) {
	s.onLeaving = append(s.onLeaving, f)
}

func (s *Service) memberLeft(ctx context.Context, guildID, memberID, actorID uint64) error {
	for _, f := range s.onLeaving {
		if err := f(ctx, guildID, memberID, actorID); err != nil {
			return err
		}
	}
	return nil
}

// hirerRank is the Rank of agent's Hirer in h's Guild, 0 when they hold no
// Membership there any more.
func (s *Service) hirerRank(ctx context.Context, h Hierarchy, agent domain.Membership) (domain.Rank, error) {
	hirer, ok := membershipOf(h.Memberships, agent.HirerID)
	if !ok {
		return 0, nil
	}
	admin, err := s.members.IsInstanceAdmin(ctx, agent.HirerID)
	if err != nil {
		return 0, err
	}
	return domain.RankOf(h.Guild, h.Roles, hirer, agent.HirerID, admin), nil
}

// keepAgentsBelowHirers adds to c, for every Agent membership of h's
// Guild, the removal of the Roles at or above its Hirer's highest as they
// stand once c is made. These removals record no Activity.
func (s *Service) keepAgentsBelowHirers(ctx context.Context, h Hierarchy, c *Change) error {
	after := afterChange(h, *c)
	for _, agent := range after.Memberships {
		if !agent.IsAgent() {
			continue
		}
		hirer, err := s.hirerRank(ctx, after, agent)
		if err != nil {
			return err
		}
		gone := domain.AgentRolesAbove(after.Roles, agent, int(hirer))
		if len(gone) == 0 {
			continue
		}
		agent.RoleIDs = slices.DeleteFunc(slices.Clone(agent.RoleIDs), func(id uint64) bool { return slices.Contains(gone, id) })
		if i := slices.IndexFunc(c.Agents, func(m domain.Membership) bool { return m.AgentID == agent.AgentID }); i >= 0 {
			c.Agents[i].RoleIDs = agent.RoleIDs
		} else {
			c.Agents = append(c.Agents, agent)
		}
	}
	return nil
}

// afterChange is h as it stands once c is stored. Created Roles are left
// out: nobody holds them yet.
func afterChange(h Hierarchy, c Change) Hierarchy {
	out := Hierarchy{Guild: h.Guild}
	for _, r := range h.Roles {
		if r.ID == c.DeletedRole {
			continue
		}
		if i := slices.IndexFunc(c.Roles, func(x domain.Role) bool { return x.ID == r.ID }); i >= 0 {
			r = c.Roles[i]
		}
		out.Roles = append(out.Roles, r)
	}
	for _, m := range h.Memberships {
		if m.ID == c.RemovedMembership {
			continue
		}
		if c.Membership != nil && c.Membership.ID == m.ID {
			m = *c.Membership
		}
		if i := slices.IndexFunc(c.Agents, func(x domain.Membership) bool { return x.AgentID == m.AgentID }); m.IsAgent() && i >= 0 {
			m = c.Agents[i]
		}
		out.Memberships = append(out.Memberships, m)
	}
	for _, m := range c.Agents {
		if m.ID == 0 {
			out.Memberships = append(out.Memberships, m)
		}
	}
	return out
}

// agentOf is agentID's Agent membership among ms.
func agentOf(ms []domain.Membership, agentID uint64) (domain.Membership, bool) {
	for _, m := range ms {
		if m.IsAgent() && m.AgentID == agentID {
			return m, true
		}
	}
	return domain.Membership{}, false
}
