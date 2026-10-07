package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

var ErrMembershipNotFound = errors.New("member not found")

// Place is the Guild a request acts in and what the Member may do there.
type Place struct {
	Guild       domain.Guild
	Permissions domain.Permissions
	// GuildMaster is set when the Member is the Guild's Guild Master.
	GuildMaster bool
}

// Place finds the Current guild of a Member's request: for an API token
// (tokenGuild set) the Guild it was made in; for a Session the wanted one
// (the Guild cookie) when the Member is in it, else their first Guild by
// id. The Guild Master holds every Permission in their Guild, the Instance
// admin in every Guild, and with no Membership at all acts in the first
// Guild there is. False when there is no Guild to act
// in.
func (s *Service) Place(ctx context.Context, memberID uint64, instanceAdmin bool, tokenGuild, wanted uint64) (Place, bool, error) {
	if tokenGuild != 0 {
		return s.placeIn(ctx, tokenGuild, memberID, instanceAdmin)
	}
	if wanted != 0 {
		p, ok, err := s.placeIn(ctx, wanted, memberID, instanceAdmin)
		if err != nil || ok {
			return p, ok, err
		}
	}
	ms, err := s.memberships.ListForMember(ctx, memberID)
	if err != nil {
		return Place{}, false, err
	}
	if len(ms) > 0 {
		return s.placeIn(ctx, ms[0].GuildID, memberID, instanceAdmin)
	}
	if instanceAdmin {
		all, err := s.guilds.All(ctx)
		if err != nil || len(all) == 0 {
			return Place{}, false, err
		}
		return Place{Guild: all[0], Permissions: domain.AllPermissions, GuildMaster: all[0].MasterID == memberID}, true, nil
	}
	return Place{}, false, nil
}

func (s *Service) placeIn(ctx context.Context, guildID, memberID uint64, instanceAdmin bool) (Place, bool, error) {
	g, found, err := s.guilds.ByID(ctx, guildID)
	if err != nil || !found {
		return Place{}, false, err
	}
	master := g.MasterID == memberID
	if instanceAdmin || master {
		return Place{Guild: g, Permissions: domain.AllPermissions, GuildMaster: master}, true, nil
	}
	perms, ok, err := s.PermissionsIn(ctx, guildID, memberID)
	if err != nil || !ok {
		return Place{}, false, err
	}
	return Place{Guild: g, Permissions: perms}, true, nil
}

// GuildsFor lists the Guilds a Member may act in, by id, with their
// Permissions in each: every Guild, with every Permission, for the Instance
// admin.
func (s *Service) GuildsFor(ctx context.Context, memberID uint64, instanceAdmin bool) ([]Place, error) {
	if instanceAdmin {
		all, err := s.guilds.All(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Place, len(all))
		for i, g := range all {
			out[i] = Place{Guild: g, Permissions: domain.AllPermissions, GuildMaster: g.MasterID == memberID}
		}
		return out, nil
	}
	ms, err := s.memberships.ListForMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]Place, 0, len(ms))
	for _, m := range ms {
		g, found, err := s.guilds.ByID(ctx, m.GuildID)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if g.MasterID == memberID {
			out = append(out, Place{Guild: g, Permissions: domain.AllPermissions, GuildMaster: true})
			continue
		}
		roles, err := s.roles.ForGuild(ctx, m.GuildID)
		if err != nil {
			return nil, err
		}
		out = append(out, Place{Guild: g, Permissions: domain.PermissionsOf(roles, m)})
	}
	return out, nil
}

// MembershipsIn lists the Guild's Memberships.
func (s *Service) MembershipsIn(ctx context.Context, guildID uint64) ([]domain.Membership, error) {
	return s.memberships.ListForGuild(ctx, guildID)
}

// RemoveMembership takes a Member out of the Guild by the hierarchy's
// rules; their API tokens of the Guild and a Transfer offer to them go with
// it, and they keep their account and other Memberships.
func (s *Service) RemoveMembership(ctx context.Context, guildID, actorID uint64, actor domain.Permissions, memberID uint64) error {
	_, err := s.manage(ctx, guildID, actorID, actor, domain.PermissionManageMembers, memberID, func(_ Hierarchy, _ domain.Actor, target domain.Membership) (Change, error) {
		return Change{RemovedMembership: target.ID}, nil
	})
	if err != nil {
		return err
	}
	if err := s.offers.WithdrawTo(ctx, guildID, memberID); err != nil {
		return err
	}
	return s.members.RevokeAPITokens(ctx, memberID, guildID)
}

// ResetTwoFactor switches off the two-factor of a Member of the Guild who
// is locked out, by the same rules as removing them.
func (s *Service) ResetTwoFactor(ctx context.Context, guildID, actorID uint64, actor domain.Permissions, memberID uint64) error {
	g, err := s.Guild(ctx, guildID)
	if err != nil {
		return err
	}
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return err
	}
	ms, err := s.memberships.ListForGuild(ctx, guildID)
	if err != nil {
		return err
	}
	h := Hierarchy{Guild: g, Roles: roles, Memberships: ms}
	a, err := s.actor(ctx, h, actorID, actor)
	if err != nil {
		return err
	}
	if err := s.canManage(ctx, h, a, domain.PermissionManageMembers, memberID); err != nil {
		return err
	}
	return s.members.ResetTwoFactor(ctx, memberID)
}

// actor is who asks, with these Permissions, for a change in h's Guild.
func (s *Service) actor(ctx context.Context, h Hierarchy, actorID uint64, perms domain.Permissions) (domain.Actor, error) {
	instanceAdmin, err := s.members.IsInstanceAdmin(ctx, actorID)
	if err != nil {
		return domain.Actor{}, err
	}
	m, _ := membershipOf(h.Memberships, actorID)
	return domain.Actor{MemberID: actorID, Permissions: perms, Rank: domain.RankOf(h.Guild, h.Roles, m, actorID, instanceAdmin)}, nil
}

// canManage is domain.CanManage for memberID's Membership in h's Guild;
// ErrMembershipNotFound without one.
func (s *Service) canManage(ctx context.Context, h Hierarchy, a domain.Actor, need domain.Permission, memberID uint64) error {
	target, ok := membershipOf(h.Memberships, memberID)
	if !ok {
		return ErrMembershipNotFound
	}
	instanceAdmin, err := s.members.IsInstanceAdmin(ctx, memberID)
	if err != nil {
		return err
	}
	return domain.CanManage(h.Guild, a, need, target, domain.RankOf(h.Guild, h.Roles, target, memberID, instanceAdmin), instanceAdmin)
}

// manage changes memberID's Membership in the Guild with change once
// domain.CanManage allows it, with the Guild locked.
func (s *Service) manage(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, need domain.Permission, memberID uint64, change func(Hierarchy, domain.Actor, domain.Membership) (Change, error)) (Change, error) {
	return s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		if err := s.canManage(ctx, h, a, need, memberID); err != nil {
			return Change{}, err
		}
		target, _ := membershipOf(h.Memberships, memberID)
		return change(h, a, target)
	})
}

// changeRoles is Roles.Change with the actor asking for it.
func (s *Service) changeRoles(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, decide func(Hierarchy, domain.Actor) (Change, error)) (Change, error) {
	return s.roles.Change(ctx, guildID, func(h Hierarchy) (Change, error) {
		a, err := s.actor(ctx, h, actorID, perms)
		if err != nil {
			return Change{}, err
		}
		return decide(h, a)
	})
}

func membershipOf(ms []domain.Membership, memberID uint64) (domain.Membership, bool) {
	for _, m := range ms {
		if m.MemberID == memberID {
			return m, true
		}
	}
	return domain.Membership{}, false
}
