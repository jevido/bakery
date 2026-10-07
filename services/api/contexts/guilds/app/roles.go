package app

import (
	"context"
	"errors"
	"slices"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

var ErrRoleNotFound = errors.New("role not found")

// RoleEdit is what EditRole changes; nil leaves it as it is.
type RoleEdit struct {
	Name        *string
	Color       *string
	Permissions *domain.Permissions
}

// CreateRole makes a Role in the Guild just above the Base role, moving
// every other Role up one, as Discord places a new Role. Its Permissions
// are only ones the actor holds.
func (s *Service) CreateRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, name, color string, permissions domain.Permissions) (domain.Role, error) {
	r, err := domain.NewRole(guildID, name, color, permissions)
	if err != nil {
		return domain.Role{}, err
	}
	c, err := s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		if err := mayChangeRoles(a); err != nil {
			return Change{}, err
		}
		if err := domain.CanGrant(a.Permissions, 0, r.Permissions); err != nil {
			return Change{}, err
		}
		r.Position = 1
		return Change{Roles: append(domain.PlaceNew(h.Roles), r)}, nil
	})
	if err != nil {
		return domain.Role{}, err
	}
	return c.Roles[len(c.Roles)-1], nil
}

// EditRole renames, recolors or changes the Permissions of a Role below
// the actor's highest; of the Base role only its Permissions.
func (s *Service) EditRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, roleID uint64, edit RoleEdit) (domain.Role, error) {
	c, err := s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		r, err := roleIn(h, a, roleID, domain.CanEditRole)
		if err != nil {
			return Change{}, err
		}
		if edit.Name != nil {
			if err := r.Rename(*edit.Name); err != nil {
				return Change{}, err
			}
		}
		if edit.Color != nil {
			if err := r.Recolor(*edit.Color); err != nil {
				return Change{}, err
			}
		}
		if edit.Permissions != nil {
			if err := domain.CanGrant(a.Permissions, r.Permissions, *edit.Permissions); err != nil {
				return Change{}, err
			}
			r.Permissions = *edit.Permissions
		}
		return Change{Roles: []domain.Role{r}}, nil
	})
	if err != nil {
		return domain.Role{}, err
	}
	return c.Roles[0], nil
}

// DeleteRole deletes a Role below the actor's highest, never the Base
// role; the Members holding it stop holding it, and the Roles above it move
// down one. The seeded Roles are ordinary once seeded.
func (s *Service) DeleteRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, roleID uint64) error {
	_, err := s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		r, err := roleIn(h, a, roleID, domain.CanDeleteRole)
		if err != nil {
			return Change{}, err
		}
		return Change{DeletedRole: r.ID, Roles: domain.CloseGap(h.Roles, r)}, nil
	})
	return err
}

// ReorderRoles places the Guild's Roles in order (ids, top first, every
// Role but the Base role), moving only Roles below the actor's highest,
// and lists the Guild's Roles as they are then.
func (s *Service) ReorderRoles(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, order []uint64) ([]domain.Role, error) {
	_, err := s.changeRoles(ctx, guildID, actorID, perms, func(h Hierarchy, a domain.Actor) (Change, error) {
		if err := mayChangeRoles(a); err != nil {
			return Change{}, err
		}
		moved, err := domain.Reorder(h.Roles, order, a.Rank)
		return Change{Roles: moved}, err
	})
	if err != nil {
		return nil, err
	}
	return s.roles.ForGuild(ctx, guildID)
}

// AssignRole gives a Member of the Guild a Role below the actor's highest,
// when the Member's own highest is below it too.
func (s *Service) AssignRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, memberID, roleID uint64) (domain.Membership, error) {
	return s.reRole(ctx, guildID, actorID, perms, memberID, roleID, func(held []uint64) []uint64 {
		if slices.Contains(held, roleID) {
			return held
		}
		return append(slices.Clone(held), roleID)
	})
}

// RemoveRole takes a Role from a Member of the Guild by AssignRole's rules.
func (s *Service) RemoveRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, memberID, roleID uint64) (domain.Membership, error) {
	return s.reRole(ctx, guildID, actorID, perms, memberID, roleID, func(held []uint64) []uint64 {
		return slices.DeleteFunc(slices.Clone(held), func(id uint64) bool { return id == roleID })
	})
}

func (s *Service) reRole(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, memberID, roleID uint64, to func([]uint64) []uint64) (domain.Membership, error) {
	c, err := s.manage(ctx, guildID, actorID, perms, domain.PermissionManageRoles, memberID, func(h Hierarchy, a domain.Actor, target domain.Membership) (Change, error) {
		if _, err := roleIn(h, a, roleID, domain.CanAssign); err != nil {
			return Change{}, err
		}
		target.RoleIDs = to(target.RoleIDs)
		return Change{Membership: &target}, nil
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return *c.Membership, nil
}

func mayChangeRoles(a domain.Actor) error {
	if !a.Permissions.Has(domain.PermissionManageRoles) {
		return domain.ErrMissing{Permission: domain.PermissionManageRoles}
	}
	return nil
}

// roleIn is the Role of h's Guild with this id once a, with manage_roles,
// passes rule for it; ErrRoleNotFound for another Guild's.
func roleIn(h Hierarchy, a domain.Actor, id uint64, rule func(domain.Rank, domain.Role) error) (domain.Role, error) {
	if err := mayChangeRoles(a); err != nil {
		return domain.Role{}, err
	}
	r, ok := findRole(h.Roles, id)
	if !ok {
		return domain.Role{}, ErrRoleNotFound
	}
	return r, rule(a.Rank, r)
}

func findRole(roles []domain.Role, id uint64) (domain.Role, bool) {
	for _, r := range roles {
		if r.ID == id {
			return r, true
		}
	}
	return domain.Role{}, false
}
