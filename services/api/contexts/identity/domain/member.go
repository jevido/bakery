// Package domain is the identity model: Members, their API tokens and
// Two-factor authentication, and the rules for them.
package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// MinPasswordLength is the shortest password Setup accepts.
const MinPasswordLength = 12

var (
	ErrInvalidName      = errors.New("name is required")
	ErrInvalidEmail     = errors.New("email is not a valid address")
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
)

// Member is a person who may sign in to this Bakery.
type Member struct {
	ID           uint64
	Name         string
	Email        string
	PasswordHash string
	// InstanceAdmin marks the one Member Setup created: admin in every
	// Guild.
	InstanceAdmin bool
	TwoFactor     TwoFactor
	// SessionsValidFrom is the moment before which the Member's Sessions
	// no longer count; zero when every Session counts.
	SessionsValidFrom time.Time
}

// NewMember validates a new Member and returns it without an ID or password
// hash; the caller hashes the password it validated here.
func NewMember(name, email, password string) (Member, error) {
	name, err := ValidateName(name)
	if err != nil {
		return Member{}, err
	}
	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return Member{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Member{}, err
	}
	return Member{Name: name, Email: email, TwoFactor: TwoFactor{State: TwoFactorOff}}, nil
}

// ValidateName trims a Member's name and refuses an empty one.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidName
	}
	return name, nil
}

// ValidatePassword refuses a password shorter than MinPasswordLength.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// SessionsValidFromNow is the Sessions valid from stamp for "now": whole
// seconds, like a Session's issue time, so a Session issued in this same
// second (the fresh one of the request that set the stamp) still counts.
func SessionsValidFromNow(now time.Time) time.Time { return now.Truncate(time.Second) }

// SessionCounts reports whether a Session issued at issuedAt still counts.
func (m Member) SessionCounts(issuedAt time.Time) bool {
	return !issuedAt.Before(m.SessionsValidFrom)
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
