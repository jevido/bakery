package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewInvitation(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	i, err := NewInvitation(" Dev@Example.com", RoleMember, 1, now)
	if err != nil || i.Email != "dev@example.com" || !i.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("got %+v, %v", i, err)
	}
	if _, err := NewInvitation("dev@example.com", Role("owner"), 1, now); !errors.Is(err, ErrInvitationRole) {
		t.Errorf("owner role: %v", err)
	}
	if _, err := NewInvitation("nope", RoleMember, 1, now); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("bad email: %v", err)
	}
}

func TestAcceptInvitation(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	i, _ := NewInvitation("dev@example.com", RoleViewer, 1, now)
	if err := i.Accept(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := i.Accept(now.Add(2 * time.Hour)); !errors.Is(err, ErrInvitationUsed) {
		t.Errorf("second accept: %v", err)
	}
	late, _ := NewInvitation("dev@example.com", RoleViewer, 1, now)
	if err := late.Accept(now.Add(InvitationLifetime)); !errors.Is(err, ErrInvitationExpired) {
		t.Errorf("expired: %v", err)
	}
	revoked, _ := NewInvitation("dev@example.com", RoleViewer, 1, now)
	revoked.RevokedAt = &now
	if revoked.Open(now) {
		t.Error("revoked invitation is open")
	}
}
