package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

var ErrMembershipNotFound = errors.New("member not found")

// Place is the Guild a request acts in and the Member's Role there.
type Place struct {
	Guild domain.Guild
	Role  domain.Role
}

// Place finds the Current guild of a Member's request: for an API token
// (tokenGuild set) the Guild it was made in; for a Session the wanted one
// (the Guild cookie) when the Member is in it, else their first Guild by
// id. The Instance admin is admin in every Guild, and with no Membership at
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
		return Place{Guild: all[0], Role: domain.RoleAdmin}, true, nil
	}
	return Place{}, false, nil
}

func (s *Service) placeIn(ctx context.Context, guildID, memberID uint64, instanceAdmin bool) (Place, bool, error) {
	g, found, err := s.guilds.ByID(ctx, guildID)
	if err != nil || !found {
		return Place{}, false, err
	}
	if instanceAdmin {
		return Place{Guild: g, Role: domain.RoleAdmin}, true, nil
	}
	role, ok, err := s.memberships.RoleOf(ctx, guildID, memberID)
	if err != nil || !ok {
		return Place{}, false, err
	}
	return Place{Guild: g, Role: role}, true, nil
}

// GuildsFor lists the Guilds a Member may act in, by id, with their Role in
// each: every Guild, as admin, for the Instance admin.
func (s *Service) GuildsFor(ctx context.Context, memberID uint64, instanceAdmin bool) ([]Place, error) {
	if instanceAdmin {
		all, err := s.guilds.All(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Place, len(all))
		for i, g := range all {
			out[i] = Place{Guild: g, Role: domain.RoleAdmin}
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
		if found {
			out = append(out, Place{Guild: g, Role: m.Role})
		}
	}
	return out, nil
}

// MembershipsIn lists the Guild's Memberships.
func (s *Service) MembershipsIn(ctx context.Context, guildID uint64) ([]domain.Membership, error) {
	return s.memberships.ListForGuild(ctx, guildID)
}

// ChangeRole gives a Member another Role in the Guild, by the domain's
// rules; the Guild keeps an admin.
func (s *Service) ChangeRole(ctx context.Context, guildID, actorID uint64, actorRole domain.Role, memberID uint64, to domain.Role) (domain.Membership, error) {
	if _, err := domain.ParseRole(string(to)); err != nil {
		return domain.Membership{}, err
	}
	return s.change(ctx, guildID, actorID, actorRole, memberID, to)
}

// RemoveMembership takes a Member out of the Guild; their API tokens of the
// Guild go with it, and they keep their account and other Memberships.
func (s *Service) RemoveMembership(ctx context.Context, guildID, actorID uint64, actorRole domain.Role, memberID uint64) error {
	if _, err := s.change(ctx, guildID, actorID, actorRole, memberID, ""); err != nil {
		return err
	}
	return s.members.RevokeAPITokens(ctx, memberID, guildID)
}

func (s *Service) change(ctx context.Context, guildID, actorID uint64, actorRole domain.Role, memberID uint64, to domain.Role) (domain.Membership, error) {
	instanceAdmin, err := s.members.IsInstanceAdmin(ctx, memberID)
	if err != nil {
		return domain.Membership{}, err
	}
	return s.memberships.Change(ctx, guildID, memberID, to, func(ms []domain.Membership) error {
		for _, m := range ms {
			if m.MemberID == memberID {
				if err := domain.CanManage(actorID, actorRole, m, instanceAdmin); err != nil {
					return err
				}
			}
		}
		return domain.KeepsAnAdmin(ms, memberID, to)
	})
}

// ResetTwoFactor switches off the two-factor of a Member of the Guild who
// is locked out, by the same rules as changing their Role.
func (s *Service) ResetTwoFactor(ctx context.Context, guildID, actorID uint64, actorRole domain.Role, memberID uint64) error {
	role, ok, err := s.memberships.RoleOf(ctx, guildID, memberID)
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
	target := domain.Membership{GuildID: guildID, MemberID: memberID, Role: role}
	if err := domain.CanManage(actorID, actorRole, target, instanceAdmin); err != nil {
		return err
	}
	return s.members.ResetTwoFactor(ctx, memberID)
}

// MemberAdded answers identity's MemberAdded: the Instance admin Setup made
// gets the first Guild; someone who accepted an Invitation gets a
// Membership with its Role in the first Guild (Invitations name their
// Guild once guilds owns them).
func (s *Service) MemberAdded(ctx context.Context, memberID uint64, instanceAdmin bool, role string) error {
	if instanceAdmin {
		return s.MakeFirstGuild(ctx, memberID)
	}
	r, err := domain.ParseRole(role)
	if err != nil {
		return err
	}
	all, err := s.guilds.All(ctx)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return errors.New("there is no guild to join")
	}
	return s.memberships.Add(ctx, domain.Membership{GuildID: all[0].ID, MemberID: memberID, Role: r})
}
