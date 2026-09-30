// Package domain is the identity model: Members, their Roles, and the rules
// for them.
package domain

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// MinPasswordLength is the shortest password Setup accepts.
const MinPasswordLength = 12

var (
	ErrInvalidName      = errors.New("name is required")
	ErrInvalidEmail     = errors.New("email is not a valid address")
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
	ErrInvalidRole      = errors.New("role must be viewer, member, admin or owner")
)

// Role is what a Member may do.
type Role string

const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
	RoleOwner  Role = "owner"
)

// ParseRole accepts the four Roles by name.
func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleViewer, RoleMember, RoleAdmin, RoleOwner:
		return r, nil
	}
	return "", ErrInvalidRole
}

// CanWrite reports whether the Role may change anything at all.
func (r Role) CanWrite() bool { return r == RoleMember || r.IsAdmin() }

// CanSeeSecrets reports whether the Role may read Secrets.
func (r Role) CanSeeSecrets() bool { return r.CanWrite() }

// IsAdmin reports whether the Role manages Servers, S3 storages, Known hosts
// and Members.
func (r Role) IsAdmin() bool { return r == RoleAdmin || r == RoleOwner }

// Member is a person who may sign in to this Bakery.
type Member struct {
	ID           uint64
	Name         string
	Email        string
	PasswordHash string
	Role         Role
}

// NewMember validates a new Member and returns it without an ID or password
// hash; the caller hashes the password it validated here.
func NewMember(name, email, password string, role Role) (Member, error) {
	name = strings.TrimSpace(name)
	email = NormalizeEmail(email)
	if name == "" {
		return Member{}, ErrInvalidName
	}
	if err := ValidateEmail(email); err != nil {
		return Member{}, err
	}
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return Member{}, ErrPasswordTooShort
	}
	if _, err := ParseRole(string(role)); err != nil {
		return Member{}, err
	}
	return Member{Name: name, Email: email, Role: role}, nil
}

// ValidateEmail accepts a bare, already normalised address.
func ValidateEmail(email string) error {
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		return ErrInvalidEmail
	}
	return nil
}

// NormalizeEmail is how emails are compared and stored.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
