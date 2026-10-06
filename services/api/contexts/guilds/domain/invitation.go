package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// InvitationLifetime is how long an Invitation's link works.
const InvitationLifetime = 7 * 24 * time.Hour

var (
	ErrInvalidEmail      = errors.New("email is not a valid address")
	ErrInvitationUsed    = errors.New("this invitation has already been accepted")
	ErrInvitationExpired = errors.New("this invitation has expired")
	ErrInvitationRevoked = errors.New("this invitation was revoked")
)

// Invitation is an email invited into a Guild with a Role.
type Invitation struct {
	ID         uint64
	GuildID    uint64
	Email      string
	Role       Role
	InvitedBy  uint64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	RevokedAt  *time.Time
}

// NewInvitation validates an Invitation into the Guild made now.
func NewInvitation(guildID uint64, email string, role Role, invitedBy uint64, now time.Time) (Invitation, error) {
	email = NormalizeEmail(email)
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		return Invitation{}, ErrInvalidEmail
	}
	if _, err := ParseRole(string(role)); err != nil {
		return Invitation{}, err
	}
	return Invitation{
		GuildID: guildID, Email: email, Role: role, InvitedBy: invitedBy,
		CreatedAt: now, ExpiresAt: now.Add(InvitationLifetime),
	}, nil
}

// NormalizeEmail is how an Invitation's email is stored and compared, the
// same way identity stores a Member's.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Refusal is why the Invitation cannot be accepted now, or nil.
func (i Invitation) Refusal(now time.Time) error {
	switch {
	case i.AcceptedAt != nil:
		return ErrInvitationUsed
	case i.RevokedAt != nil:
		return ErrInvitationRevoked
	case !now.Before(i.ExpiresAt):
		return ErrInvitationExpired
	}
	return nil
}

// Open reports whether the Invitation can still be accepted.
func (i Invitation) Open(now time.Time) bool { return i.Refusal(now) == nil }

// Accept marks the Invitation used, once.
func (i *Invitation) Accept(now time.Time) error {
	if err := i.Refusal(now); err != nil {
		return err
	}
	i.AcceptedAt = &now
	return nil
}

// Membership is the Membership accepting the Invitation gives memberID.
func (i Invitation) Membership(memberID uint64) Membership {
	return Membership{GuildID: i.GuildID, MemberID: memberID, Role: i.Role}
}
