package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// APITokenPrefix starts every API token, so a leaked one is recognisable.
const APITokenPrefix = "bky_"

var (
	ErrInvalidTokenName  = errors.New("description must be 3 to 255 characters")
	ErrUnknownPermission = errors.New("permission must be root, write, deploy, read or read:sensitive")
	ErrTokenExpiryPassed = errors.New("expiry must be in the future")
	// ErrCannotGrant is wrapped with the Token permission's name.
	ErrCannotGrant = errors.New("your permissions cannot grant")
)

// Permission is what a request made with an API token may do, in Coolify's
// abilities. The Member's current Permissions in the token's Guild cap it as
// well.
type Permission string

const (
	// PermissionRoot allows everything the Member's Permissions in the Guild
	// do.
	PermissionRoot Permission = "root"
	// PermissionWrite allows changes (any method but GET and HEAD) other
	// than deploy actions.
	PermissionWrite Permission = "write"
	// PermissionDeploy allows the deploy actions: deploy, restart, stop,
	// start, cancel and rollback.
	PermissionDeploy Permission = "deploy"
	// PermissionRead allows reading, without Secrets.
	PermissionRead Permission = "read"
	// PermissionReadSensitive allows reading Secrets as well.
	PermissionReadSensitive Permission = "read:sensitive"
)

// Permissions are every Permission, in the order a person picks them.
var Permissions = []Permission{PermissionRoot, PermissionWrite, PermissionDeploy, PermissionRead, PermissionReadSensitive}

// ParsePermission accepts the five Permissions by name.
func ParsePermission(s string) (Permission, error) {
	p := Permission(s)
	if !slices.Contains(Permissions, p) {
		return "", ErrUnknownPermission
	}
	return p, nil
}

// NormalisePermissions leaves permissions as Coolify's token form does:
// none is read, root stands alone, read:sensitive brings read, sorted and
// without duplicates.
func NormalisePermissions(permissions []Permission) ([]Permission, error) {
	out := make([]Permission, 0, len(permissions)+1)
	for _, p := range permissions {
		if _, err := ParsePermission(string(p)); err != nil {
			return nil, err
		}
		if p == PermissionRoot {
			return []Permission{PermissionRoot}, nil
		}
		out = append(out, p)
	}
	if len(out) == 0 || slices.Contains(out, PermissionReadSensitive) {
		out = append(out, PermissionRead)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// APIToken is a named secret of one Member for scripts, acting in the Guild
// it was made in. Its value is never
// part of it: only a hash is stored, and the value is shown once.
type APIToken struct {
	ID          uint64
	MemberID    uint64
	GuildID     uint64
	Name        string
	Permissions []Permission
	// ExpiresAt is nil for a token that never expires.
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// NewAPIToken validates a new API token of a Member with these Permissions
// in the Guild, which cap the Token permissions it may carry.
func NewAPIToken(memberID, guildID uint64, member MemberPermissions, name string, permissions []Permission, expiresAt *time.Time, now time.Time) (APIToken, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 3 || n > 255 {
		return APIToken{}, ErrInvalidTokenName
	}
	permissions, err := NormalisePermissions(permissions)
	if err != nil {
		return APIToken{}, err
	}
	for _, p := range permissions {
		if !member.MayGrant(p) {
			return APIToken{}, fmt.Errorf("%w %s", ErrCannotGrant, p)
		}
	}
	if expiresAt != nil && !expiresAt.After(now) {
		return APIToken{}, ErrTokenExpiryPassed
	}
	return APIToken{MemberID: memberID, GuildID: guildID, Name: name, Permissions: permissions, ExpiresAt: expiresAt, CreatedAt: now}, nil
}

// Expired reports whether the token no longer authenticates at now.
func (t APIToken) Expired(now time.Time) bool {
	return t.ExpiresAt != nil && !now.Before(*t.ExpiresAt)
}

// Allows reports whether the token carries p, or root.
func (t APIToken) Allows(p Permission) bool {
	return slices.Contains(t.Permissions, PermissionRoot) || slices.Contains(t.Permissions, p)
}

// ReadOnly reports whether the token only reads, as the read-only tokens
// before Permissions did.
func (t APIToken) ReadOnly() bool {
	return slices.Equal(t.Permissions, []Permission{PermissionRead})
}
