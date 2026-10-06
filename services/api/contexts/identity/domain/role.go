package domain

import "errors"

var ErrInvalidRole = errors.New("role must be viewer, member or admin")

// Role is what a Member may do in the Guild a request acts in. Guilds holds
// the Roles; identity is told the one that counts and uses it only to cap
// an API token's Permissions.
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

// IsAdmin reports whether the Role is admin.
func (r Role) IsAdmin() bool { return r == RoleAdmin }
