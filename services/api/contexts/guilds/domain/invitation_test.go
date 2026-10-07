package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewInvitation(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	i, err := NewInvitation(3, " Dev@Example.com", []uint64{7}, 1, now)
	if err != nil || i.GuildID != 3 || i.Email != "dev@example.com" || !i.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("got %+v, %v", i, err)
	}
	if _, err := NewInvitation(3, "nope", []uint64{7}, 1, now); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("bad email: %v", err)
	}
}

func TestAcceptInvitation(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	i, _ := NewInvitation(1, "dev@example.com", nil, 1, now)
	if err := i.Accept(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := i.Accept(now.Add(2 * time.Hour)); !errors.Is(err, ErrInvitationUsed) {
		t.Errorf("second accept: %v", err)
	}
	late, _ := NewInvitation(1, "dev@example.com", nil, 1, now)
	if err := late.Accept(now.Add(InvitationLifetime)); !errors.Is(err, ErrInvitationExpired) {
		t.Errorf("expired: %v", err)
	}
	revoked, _ := NewInvitation(1, "dev@example.com", nil, 1, now)
	revoked.RevokedAt = &now
	if revoked.Open(now) {
		t.Error("revoked invitation is open")
	}
}
