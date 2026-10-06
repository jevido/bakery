package domain

import (
	"errors"
	"slices"
)

var (
	ErrLastAdmin          = errors.New("a guild keeps at least one admin")
	ErrNotAdmin           = errors.New("only admins can do this")
	ErrInstanceAdminFixed = errors.New("the instance admin's role cannot change and they cannot be removed")
	ErrSelf               = errors.New("you cannot change your own role or remove yourself")
)

// Membership is a Member's place in a Guild: the Roles they hold there
// besides the Base role, which every Member holds.
type Membership struct {
	ID       uint64
	GuildID  uint64
	MemberID uint64
	RoleIDs  []uint64
}

// PermissionsOf is what m allows: the union of the Base role's Permissions
// and those of every Role m holds. roles are the Guild's Roles.
func PermissionsOf(roles []Role, m Membership) Permissions {
	var out Permissions
	for _, r := range roles {
		if r.Base || slices.Contains(m.RoleIDs, r.ID) {
			out = out.Union(r.Permissions)
		}
	}
	return out
}

// KeepsAnAdmin checks that someone in these Memberships of one Guild, as
// they would be after a change, still holds administrator.
func KeepsAnAdmin(roles []Role, after []Membership) error {
	for _, m := range after {
		if PermissionsOf(roles, m).Has(PermissionAdministrator) {
			return nil
		}
	}
	return ErrLastAdmin
}

// CanManage says whether an actor with these Permissions in a Guild may
// change target's Roles there, remove target's Membership or reset their
// two-factor: only with manage_members, never for the Instance admin,
// never for themselves.
func CanManage(actorID uint64, actor Permissions, target Membership, targetIsInstanceAdmin bool) error {
	switch {
	case !actor.Has(PermissionManageMembers):
		return ErrNotAdmin
	case targetIsInstanceAdmin:
		return ErrInstanceAdminFixed
	case actorID == target.MemberID:
		return ErrSelf
	}
	return nil
}
