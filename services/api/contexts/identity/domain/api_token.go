package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// APITokenPrefix starts every API token, so a leaked one is recognisable.
const APITokenPrefix = "bky_"

var ErrInvalidTokenName = errors.New("name must be 1 to 64 characters")

// APIToken is a named secret of one Member for scripts. Its value is never
// part of it: only a hash is stored, and the value is shown once.
type APIToken struct {
	ID         uint64
	MemberID   uint64
	Name       string
	ReadOnly   bool
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// NewAPIToken validates a new API token of member.
func NewAPIToken(memberID uint64, name string, readOnly bool, now time.Time) (APIToken, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > 64 {
		return APIToken{}, ErrInvalidTokenName
	}
	return APIToken{MemberID: memberID, Name: name, ReadOnly: readOnly, CreatedAt: now}, nil
}

// EffectiveRole is the Role a request made with the token acts with: the
// Member's, or viewer when the token is read-only.
func (t APIToken) EffectiveRole(memberRole Role) Role {
	if t.ReadOnly {
		return RoleViewer
	}
	return memberRole
}
