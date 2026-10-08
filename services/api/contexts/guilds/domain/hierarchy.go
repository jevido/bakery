package domain

import (
	"errors"
	"math"
	"slices"
)

var (
	ErrRoleNotBelow   = errors.New("you can only change roles below your highest role")
	ErrBaseRoleFixed  = errors.New("the @everyone role cannot be renamed, recolored, moved, deleted, assigned or removed")
	ErrMemberNotBelow = errors.New("you can only manage members whose highest role is below yours")
	ErrNotHeld        = errors.New("you can only grant permissions you hold")
	ErrInvalidOrder   = errors.New("the order must name every role except @everyone once")
	ErrAboveHirer     = errors.New("an agent cannot hold a role at or above its hirer's highest role")
)

// ErrMissing refuses an actor without the Permission an action needs.
type ErrMissing struct{ Permission Permission }

func (e ErrMissing) Error() string { return "you need the " + e.Permission.Name() + " permission" }

// Rank is where someone stands in a Guild's hierarchy: the Position of
// their highest Role (0 with only the Base role), or above every Role for
// the Instance admin, and above that for the Guild Master.
type Rank int

const (
	InstanceAdminRank Rank = math.MaxInt - 1
	GuildMasterRank   Rank = math.MaxInt
)

// Above reports whether the Rank is above a Role at position.
func (r Rank) Above(position int) bool { return int(r) > position }

// RankOf is memberID's Rank in g, whose Roles are roles; m is their
// Membership there (none for an Instance admin without one).
func RankOf(g Guild, roles []Role, m Membership, memberID uint64, instanceAdmin bool) Rank {
	switch {
	case g.MasterID == memberID:
		return GuildMasterRank
	case instanceAdmin:
		return InstanceAdminRank
	}
	return Rank(HighestPosition(roles, m))
}

// HighestPosition is the Position of the highest Role m holds, 0 with only
// the Base role.
func HighestPosition(roles []Role, m Membership) int {
	top := 0
	for _, r := range roles {
		if !r.Base && slices.Contains(m.RoleIDs, r.ID) && r.Position > top {
			top = r.Position
		}
	}
	return top
}

// Actor is who asks for a change in a Guild: their Permissions there (an
// API token's cap included) and their Rank.
type Actor struct {
	MemberID    uint64
	Permissions Permissions
	Rank        Rank
}

// CanEditRole says whether actor may change role: only a Role below their
// highest. The Base role is below everyone, but only its Permissions
// change, as Discord's @everyone (Rename and Recolor refuse it).
func CanEditRole(actor Rank, role Role) error {
	if role.Base || actor.Above(role.Position) {
		return nil
	}
	return ErrRoleNotBelow
}

// CanDeleteRole is CanEditRole, never for the Base role.
func CanDeleteRole(actor Rank, role Role) error {
	if role.Base {
		return ErrBaseRoleFixed
	}
	return CanEditRole(actor, role)
}

// CanAssign says whether actor may give or take role: only a Role below
// their highest, never the Base role, which every Member holds.
func CanAssign(actor Rank, role Role) error {
	if role.Base {
		return ErrBaseRoleFixed
	}
	return CanEditRole(actor, role)
}

// CanAssignToAgent says whether actor may give an Agent role: CanAssign,
// and role must rank below hirerHighest, the Rank of the Agent's Hirer
// (above every Role for a Guild Master or the Instance admin), so an Agent
// is never placed above the person who hired it.
func CanAssignToAgent(actor Rank, hirerHighest int, role Role) error {
	if err := CanAssign(actor, role); err != nil {
		return err
	}
	if role.Position >= hirerHighest {
		return ErrAboveHirer
	}
	return nil
}

// AgentRolesAbove is the ids of the Roles agent holds at or above
// hirerHighest, which it loses when its Hirer drops there.
func AgentRolesAbove(roles []Role, agent Membership, hirerHighest int) []uint64 {
	var out []uint64
	for _, r := range roles {
		if !r.Base && r.Position >= hirerHighest && slices.Contains(agent.RoleIDs, r.ID) {
			out = append(out, r.ID)
		}
	}
	return out
}

// CanGrant says whether someone holding held may change a Role's
// Permissions from before to after: every Permission switched on or off
// must be one they hold, administrator only when they hold it (Discord's
// rule, so nobody escapes the hierarchy through a Role below their own).
func CanGrant(held, before, after Permissions) error {
	if (before^after)&^held.Expand() != 0 {
		return ErrNotHeld
	}
	return nil
}

// CanManage says whether actor may change target's Roles in g, remove
// target's Membership or reset their two-factor: only with need, never for
// themselves, g's Guild Master or the Instance admin, and only when
// target's highest Role is below actor's. The Guild Master is why a Guild
// always keeps someone with every Permission.
func CanManage(g Guild, actor Actor, need Permission, target Membership, targetRank Rank, targetIsInstanceAdmin bool) error {
	switch {
	case !actor.Permissions.Has(need):
		return ErrMissing{Permission: need}
	case targetIsInstanceAdmin:
		return ErrInstanceAdminFixed
	case actor.MemberID == target.MemberID:
		return ErrSelf
	case target.MemberID == g.MasterID:
		return ErrGuildMaster
	case !actor.Rank.Above(int(targetRank)):
		return ErrMemberNotBelow
	}
	return nil
}

// nonBase is roles without the Base role, top first.
func nonBase(roles []Role) []Role {
	out := make([]Role, 0, len(roles))
	for _, r := range roles {
		if !r.Base {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b Role) int { return b.Position - a.Position })
	return out
}

// Reorder places a Guild's Roles in order (Role ids, top first, every Role
// but the Base role once) and answers them with their new Positions,
// contiguous from 1 at the bottom; the Base role stays at 0. Only Roles
// below actor move: every other one must keep its place.
func Reorder(roles []Role, order []uint64, actor Rank) ([]Role, error) {
	current := nonBase(roles)
	if len(order) != len(current) {
		return nil, ErrInvalidOrder
	}
	byID := make(map[uint64]Role, len(current))
	for _, r := range current {
		byID[r.ID] = r
	}
	out := make([]Role, len(order))
	for i, id := range order {
		r, ok := byID[id]
		if !ok {
			return nil, ErrInvalidOrder
		}
		delete(byID, id)
		if id != current[i].ID && (!actor.Above(r.Position) || !actor.Above(current[i].Position)) {
			return nil, ErrRoleNotBelow
		}
		r.Position = len(order) - i
		out[i] = r
	}
	return out, nil
}

// PlaceNew answers the Roles that move up to make room for a new Role at
// Position 1, just above the Base role, where Discord puts a new Role.
func PlaceNew(roles []Role) []Role {
	var out []Role
	for _, r := range roles {
		if !r.Base {
			r.Position++
			out = append(out, r)
		}
	}
	return out
}

// CloseGap answers the Roles that move down to fill the Position a
// deleted Role leaves, so Positions stay contiguous.
func CloseGap(roles []Role, deleted Role) []Role {
	var out []Role
	for _, r := range roles {
		if !r.Base && r.Position > deleted.Position {
			r.Position--
			out = append(out, r)
		}
	}
	return out
}
