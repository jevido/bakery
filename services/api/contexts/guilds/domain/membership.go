package domain

import "errors"

var (
	ErrInvalidRole        = errors.New("role must be viewer, member or admin")
	ErrLastAdmin          = errors.New("a guild keeps at least one admin")
	ErrNotAdmin           = errors.New("only admins can do this")
	ErrInstanceAdminFixed = errors.New("the instance admin's role cannot change and they cannot be removed")
	ErrSelf               = errors.New("you cannot change your own role or remove yourself")
)

// Role is what a Member may do in one Guild.
type Role string

const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

// ParseRole accepts the three Roles by name.
func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleViewer, RoleMember, RoleAdmin:
		return r, nil
	}
	return "", ErrInvalidRole
}

// CanWrite reports whether the Role may change anything at all.
func (r Role) CanWrite() bool { return r == RoleMember || r.IsAdmin() }

// CanSeeSecrets reports whether the Role may read Secrets.
func (r Role) CanSeeSecrets() bool { return r.CanWrite() }

// IsAdmin reports whether the Role manages the Guild's Servers, S3
// storages, Known hosts and Memberships.
func (r Role) IsAdmin() bool { return r == RoleAdmin }

// Membership is a Member's place in a Guild.
type Membership struct {
	ID       uint64
	GuildID  uint64
	MemberID uint64
	Role     Role
}

// KeepsAnAdmin checks that a Guild with these Memberships still has an
// admin after memberID's Role becomes to, or after memberID leaves when to
// is empty. The caller passes every Membership of one Guild.
func KeepsAnAdmin(memberships []Membership, memberID uint64, to Role) error {
	for _, m := range memberships {
		r := m.Role
		if m.MemberID == memberID {
			r = to
		}
		if r.IsAdmin() {
			return nil
		}
	}
	return ErrLastAdmin
}

// CanManage says whether an actor with actorRole in a Guild may change
// target's Role there, remove target's Membership or reset their
// two-factor: only an admin, never for the Instance admin, never for
// themselves.
func CanManage(actorID uint64, actorRole Role, target Membership, targetIsInstanceAdmin bool) error {
	switch {
	case !actorRole.IsAdmin():
		return ErrNotAdmin
	case targetIsInstanceAdmin:
		return ErrInstanceAdminFixed
	case actorID == target.MemberID:
		return ErrSelf
	}
	return nil
}
