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
// besides the Base role, which every Member holds. An Agent membership has
// an AgentID and its Hirer's HirerID instead of a MemberID.
type Membership struct {
	ID       uint64
	GuildID  uint64
	MemberID uint64
	AgentID  uint64
	HirerID  uint64
	RoleIDs  []uint64
}

// IsAgent reports whether m is an Agent membership: it holds Roles but
// never makes anyone a Member.
func (m Membership) IsAgent() bool { return m.AgentID != 0 }

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
