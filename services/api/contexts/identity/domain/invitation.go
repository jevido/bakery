package domain

import (
	"errors"
	"time"
)

// InvitationLifetime is how long an Invitation's link works.
const InvitationLifetime = 7 * 24 * time.Hour

var (
	ErrInvitationRole    = errors.New("role must be admin, member or viewer")
	ErrInvitationUsed    = errors.New("this invitation has already been accepted")
	ErrInvitationExpired = errors.New("this invitation has expired")
	ErrInvitationRevoked = errors.New("this invitation was revoked")
)

// Invitation is an email and a Role someone was invited with.
type Invitation struct {
	ID         uint64
	Email      string
	Role       Role
	InvitedBy  uint64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	RevokedAt  *time.Time
}

// NewInvitation validates an Invitation made now. Nobody is invited as the
// Owner.
func NewInvitation(email string, role Role, invitedBy uint64, now time.Time) (Invitation, error) {
	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return Invitation{}, err
	}
	if role != RoleAdmin && role != RoleMember && role != RoleViewer {
		return Invitation{}, ErrInvitationRole
	}
	return Invitation{Email: email, Role: role, InvitedBy: invitedBy, CreatedAt: now, ExpiresAt: now.Add(InvitationLifetime)}, nil
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
