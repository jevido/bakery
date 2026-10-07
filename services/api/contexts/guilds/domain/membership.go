package domain

import (
	"errors"
	"slices"
)

var (
	ErrInstanceAdminFixed = errors.New("the instance admin's role cannot change and they cannot be removed")
	ErrSelf               = errors.New("you cannot change your own roles or remove yourself")
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
