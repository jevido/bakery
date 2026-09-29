// Package domain is the identity model: the Owner and the rules for one.
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
)

// Owner is the one person who administers this Bakery installation.
type Owner struct {
	ID           uint64
	Name         string
	Email        string
	PasswordHash string
}

// NewOwner validates what Setup was given and returns an Owner without an ID
// or password hash; the caller hashes the password it validated here.
func NewOwner(name, email, password string) (Owner, error) {
	name = strings.TrimSpace(name)
	email = NormalizeEmail(email)
	if name == "" {
		return Owner{}, ErrInvalidName
	}
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		return Owner{}, ErrInvalidEmail
	}
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return Owner{}, ErrPasswordTooShort
	}
	return Owner{Name: name, Email: email}, nil
}

// NormalizeEmail is how emails are compared and stored.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
