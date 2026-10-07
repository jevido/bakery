package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// Overrides stores the Permission overrides of every Project.
type Overrides interface {
	// ForGuild lists the Overrides of every Project of the Guild.
	ForGuild(ctx context.Context, guildID uint64) ([]domain.Override, error)
	// ForProject lists the Project's Overrides by id.
	ForProject(ctx context.Context, guildID, projectID uint64) ([]domain.Override, error)
	// ForgetProject deletes the Project's Overrides.
	ForgetProject(ctx context.Context, projectID uint64) error
}

// InProject is what someone acting at p may do in the Project: p's
// Permissions with the Project's Overrides applied.
func (s *Service) InProject(ctx context.Context, p Place, projectID uint64) (domain.Permissions, error) {
	if p.Permissions.Has(domain.PermissionAdministrator) {
		return p.Permissions, nil
	}
	os, err := s.overrides.ForProject(ctx, p.Guild.ID, projectID)
	if err != nil {
		return 0, err
	}
	return domain.Resolve(p.Permissions, os, p.BaseRoleID, p.Held, p.MemberID), nil
}

// VisibleProjects keeps the Projects of p's Guild among ids that someone
// acting at p may view, in their order, with one read of the Overrides.
func (s *Service) VisibleProjects(ctx context.Context, p Place, ids []uint64) ([]uint64, error) {
	if p.Permissions.Has(domain.PermissionAdministrator) {
		return ids, nil
	}
	os, err := s.overrides.ForGuild(ctx, p.Guild.ID)
	if err != nil {
		return nil, err
	}
	byProject := map[uint64][]domain.Override{}
	for _, o := range os {
		byProject[o.ProjectID] = append(byProject[o.ProjectID], o)
	}
	out := []uint64{}
	for _, id := range ids {
		if domain.Resolve(p.Permissions, byProject[id], p.BaseRoleID, p.Held, p.MemberID).Has(domain.PermissionViewResources) {
			out = append(out, id)
		}
	}
	return out, nil
}

// Overrides lists the Project's Overrides.
func (s *Service) Overrides(ctx context.Context, guildID, projectID uint64) ([]domain.Override, error) {
	return s.overrides.ForProject(ctx, guildID, projectID)
}

// ForgetProject deletes the Overrides of a Project that is gone.
func (s *Service) ForgetProject(ctx context.Context, projectID uint64) error {
	return s.overrides.ForgetProject(ctx, projectID)
}

// SetOverride stores what the Project allows and denies for one Role
// (roleID) or one Member (memberID); both empty deletes the Override. Only
// for a Role below the actor's highest (the Base role included) or a
// Member whose highest Role is below it, and only changing Permissions the
// actor holds in the Project (perms, resolved there).
func (s *Service) SetOverride(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, projectID, roleID, memberID uint64, allow, deny domain.Permissions) (domain.Override, error) {
	if err := domain.NewOverride(allow, deny); err != nil {
		return domain.Override{}, err
	}
	o := domain.Override{GuildID: guildID, ProjectID: projectID, RoleID: roleID, MemberID: memberID, Allow: allow, Deny: deny}
	_, err := s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		if roleID != 0 {
			if _, err := roleIn(h, a, roleID, domain.CanEditRole); err != nil {
				return Change{}, err
			}
		} else if err := s.canManage(ctx, h, a, domain.PermissionManageRoles, memberID); err != nil {
			return Change{}, err
		}
		// The Guild is locked, and every Override changes through here.
		current, err := s.overrides.ForProject(ctx, guildID, projectID)
		if err != nil {
			return Change{}, err
		}
		var before domain.Override
		for _, c := range current {
			if c.RoleID == roleID && c.MemberID == memberID {
				before = c
			}
		}
		if err := domain.CanGrant(a.Permissions, before.Allow, allow); err != nil {
			return Change{}, err
		}
		if err := domain.CanGrant(a.Permissions, before.Deny, deny); err != nil {
			return Change{}, err
		}
		return Change{Override: &o}, nil
	})
	return o, err
}
