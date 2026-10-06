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
}

// Place finds the Current guild of a Member's request: for an API token
// (tokenGuild set) the Guild it was made in; for a Session the wanted one
// (the Guild cookie) when the Member is in it, else their first Guild by
// id. The Instance admin holds every Permission in every Guild, and with no Membership at
// all acts in the first Guild there is. False when there is no Guild to act
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
		return Place{Guild: all[0], Permissions: domain.AllPermissions}, true, nil
	}
	return Place{}, false, nil
}

func (s *Service) placeIn(ctx context.Context, guildID, memberID uint64, instanceAdmin bool) (Place, bool, error) {
	g, found, err := s.guilds.ByID(ctx, guildID)
	if err != nil || !found {
		return Place{}, false, err
	}
	if instanceAdmin {
		return Place{Guild: g, Permissions: domain.AllPermissions}, true, nil
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
			out[i] = Place{Guild: g, Permissions: domain.AllPermissions}
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

// ChangeRole replaces a Member's Roles in the Guild with the seeded Role a
// former role ("viewer", "member" or "admin") names, by the domain's rules;
// the Guild keeps an admin.
func (s *Service) ChangeRole(ctx context.Context, guildID, actorID uint64, actor domain.Permissions, memberID uint64, former string) (domain.Membership, error) {
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return domain.Membership{}, err
	}
	r, found, err := domain.SeededRole(roles, former)
	if err != nil {
		return domain.Membership{}, err
	}
	if !found {
		return domain.Membership{}, domain.ErrInvalidRole
	}
	return s.change(ctx, guildID, actorID, actor, memberID, roles, []uint64{r.ID}, false)
}

// RemoveMembership takes a Member out of the Guild; their API tokens of the
// Guild go with it, and they keep their account and other Memberships.
func (s *Service) RemoveMembership(ctx context.Context, guildID, actorID uint64, actor domain.Permissions, memberID uint64) error {
	roles, err := s.roles.ForGuild(ctx, guildID)
	if err != nil {
		return err
	}
	if _, err := s.change(ctx, guildID, actorID, actor, memberID, roles, nil, true); err != nil {
		return err
	}
	return s.members.RevokeAPITokens(ctx, memberID, guildID)
}

func (s *Service) change(ctx context.Context, guildID, actorID uint64, actor domain.Permissions, memberID uint64, roles []domain.Role, to []uint64, remove bool) (domain.Membership, error) {
	instanceAdmin, err := s.members.IsInstanceAdmin(ctx, memberID)
	if err != nil {
		return domain.Membership{}, err
	}
	return s.memberships.Change(ctx, guildID, memberID, to, remove, func(ms []domain.Membership) error {
		after := make([]domain.Membership, 0, len(ms))
		for _, m := range ms {
			if m.MemberID == memberID {
				if err := domain.CanManage(actorID, actor, m, instanceAdmin); err != nil {
					return err
				}
				if remove {
					continue
				}
				m.RoleIDs = to
			}
			after = append(after, m)
		}
		return domain.KeepsAnAdmin(roles, after)
	})
}

// ResetTwoFactor switches off the two-factor of a Member of the Guild who
// is locked out, by the same rules as changing their Roles.
func (s *Service) ResetTwoFactor(ctx context.Context, guildID, actorID uint64, actor domain.Permissions, memberID uint64) error {
	target, ok, err := s.memberships.Of(ctx, guildID, memberID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMembershipNotFound
	}
	instanceAdmin, err := s.members.IsInstanceAdmin(ctx, memberID)
	if err != nil {
		return err
	}
	if err := domain.CanManage(actorID, actor, target, instanceAdmin); err != nil {
		return err
	}
	return s.members.ResetTwoFactor(ctx, memberID)
}
