package domain

import (
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DesktopSignInSecretPrefix starts the secret the Desktop app mints for a
	// Desktop sign-in and puts in the approve link.
	DesktopSignInSecretPrefix = "bky_signin_"
	// DesktopKeyPrefix starts every Desktop key, so a leaked one is
	// recognisable and never mistaken for an API token.
	DesktopKeyPrefix = "bky_desk_"
	// DesktopSignInLasts is how long a Desktop sign-in may wait for Approve.
	DesktopSignInLasts = 10 * time.Minute
	// DesktopIdleLimit is how long after it was last used a Desktop key
	// stops counting.
	DesktopIdleLimit = 30 * 24 * time.Hour
)

var (
	ErrInvalidDesktopName      = errors.New("client name must be 1 to 100 characters")
	ErrInvalidDesktopSignInKey = errors.New("token must be bky_signin_ and 48 hex characters")
	ErrInvalidDesktopKeyHash   = errors.New("desktop key hash must be a SHA-256 in hex")
	ErrDesktopSignInApproved   = errors.New("this desktop sign-in is already approved")
	ErrDesktopSignInCancelled  = errors.New("this desktop sign-in was cancelled")
	ErrDesktopSignInExpired    = errors.New("this desktop sign-in has expired")
)

// DesktopSignInStatus is where a Desktop sign-in is; it is derived from the
// times, never stored.
type DesktopSignInStatus string

const (
	DesktopSignInPending   DesktopSignInStatus = "pending"
	DesktopSignInApproved  DesktopSignInStatus = "approved"
	DesktopSignInCancelled DesktopSignInStatus = "cancelled"
	DesktopSignInExpired   DesktopSignInStatus = "expired"
)

// DesktopSignIn is the Desktop app's request to be approved in the browser.
// The app minted its secret and the Desktop key; only their hashes are
// stored, the secret's beside it by the repository.
type DesktopSignIn struct {
	ID         uint64
	ClientName string
	// KeyHash is the hash of the Desktop key the app holds; Approve gives
	// it to the new Desktop.
	KeyHash            string
	ApprovedByMemberID *uint64
	DesktopID          *uint64
	ApprovedAt         *time.Time
	CancelledAt        *time.Time
	ExpiresAt          time.Time
	CreatedAt          time.Time
}

// ValidDesktopSignInSecret reports whether s has the shape the Desktop app
// mints: the prefix and 24 random bytes in hex.
func ValidDesktopSignInSecret(s string) bool {
	rest, ok := strings.CutPrefix(s, DesktopSignInSecretPrefix)
	return ok && isHex(rest, 48)
}

// NewDesktopSignIn validates a new Desktop sign-in made at now.
func NewDesktopSignIn(clientName, keyHash string, now time.Time) (DesktopSignIn, error) {
	clientName = strings.TrimSpace(clientName)
	if n := utf8.RuneCountInString(clientName); n < 1 || n > 100 {
		return DesktopSignIn{}, ErrInvalidDesktopName
	}
	if !isHex(keyHash, 64) {
		return DesktopSignIn{}, ErrInvalidDesktopKeyHash
	}
	return DesktopSignIn{ClientName: clientName, KeyHash: keyHash, ExpiresAt: now.Add(DesktopSignInLasts), CreatedAt: now}, nil
}

// Status is cancelled, then expired once ExpiresAt has come, then approved,
// else pending: Paperclip's order, so an approved sign-in also reads expired
// after its 10 minutes.
func (s DesktopSignIn) Status(now time.Time) DesktopSignInStatus {
	switch {
	case s.CancelledAt != nil:
		return DesktopSignInCancelled
	case !now.Before(s.ExpiresAt):
		return DesktopSignInExpired
	case s.ApprovedAt != nil:
		return DesktopSignInApproved
	default:
		return DesktopSignInPending
	}
}

func (s DesktopSignIn) refuse(now time.Time) error {
	switch s.Status(now) {
	case DesktopSignInApproved:
		return ErrDesktopSignInApproved
	case DesktopSignInCancelled:
		return ErrDesktopSignInCancelled
	case DesktopSignInExpired:
		return ErrDesktopSignInExpired
	}
	return nil
}

// Approve marks a pending sign-in approved by the Member and answers the
// Desktop it makes for them, carrying the app's key.
func (s *DesktopSignIn) Approve(memberID uint64, now time.Time) (Desktop, error) {
	if err := s.refuse(now); err != nil {
		return Desktop{}, err
	}
	s.ApprovedByMemberID = &memberID
	s.ApprovedAt = &now
	return Desktop{MemberID: memberID, Name: s.ClientName, KeyHash: s.KeyHash, CreatedAt: now}, nil
}

// Cancel ends a pending sign-in.
func (s *DesktopSignIn) Cancel(now time.Time) error {
	if err := s.refuse(now); err != nil {
		return err
	}
	s.CancelledAt = &now
	return nil
}

// Desktop is one signed-in copy of the Desktop app, as the server knows it.
type Desktop struct {
	ID         uint64
	MemberID   uint64
	Name       string
	KeyHash    string
	LastSeenAt *time.Time
	// RevokedAt is when it was signed out; nil while signed in.
	RevokedAt *time.Time
	CreatedAt time.Time
}

// SignOut signs the Desktop out at now; one signed out already keeps its
// time.
func (d *Desktop) SignOut(now time.Time) {
	if d.RevokedAt == nil {
		d.RevokedAt = &now
	}
}

// Counts reports whether its key still authenticates at now: not signed
// out, and used (or made) within the last 30 days. Whether the Member's
// Sessions still count is the caller's to add.
func (d Desktop) Counts(now time.Time) bool {
	if d.RevokedAt != nil {
		return false
	}
	last := d.CreatedAt
	if d.LastSeenAt != nil && d.LastSeenAt.After(last) {
		last = *d.LastSeenAt
	}
	return now.Sub(last) < DesktopIdleLimit
}

func isHex(s string, n int) bool {
	if len(s) != n || strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
