package domain

import (
	"errors"
	"slices"
)

var (
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

// CanManage says whether an actor with these Permissions in g may change
// target's Roles there, remove target's Membership or reset their
// two-factor: only with manage_members, never for g's Guild Master or the
// Instance admin, never for themselves. The Guild Master is why a Guild
// always keeps someone with every Permission.
func CanManage(g Guild, actorID uint64, actor Permissions, target Membership, targetIsInstanceAdmin bool) error {
	switch {
	case !actor.Has(PermissionManageMembers):
		return ErrNotAdmin
	case targetIsInstanceAdmin:
		return ErrInstanceAdminFixed
	case actorID == target.MemberID:
		return ErrSelf
	case target.MemberID == g.MasterID:
		return ErrGuildMaster
	}
	return nil
}
