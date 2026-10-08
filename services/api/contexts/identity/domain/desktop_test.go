package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	t0      = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	keyHash = strings.Repeat("ab", 32)
)

func newSignIn(t *testing.T) DesktopSignIn {
	t.Helper()
	s, err := NewDesktopSignIn(" laptop ", keyHash, t0)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewDesktopSignIn(t *testing.T) {
	s := newSignIn(t)
	if s.ClientName != "laptop" || !s.ExpiresAt.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("got %+v", s)
	}
	if _, err := NewDesktopSignIn("  ", keyHash, t0); !errors.Is(err, ErrInvalidDesktopName) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := NewDesktopSignIn(strings.Repeat("x", 101), keyHash, t0); !errors.Is(err, ErrInvalidDesktopName) {
		t.Fatalf("long name: %v", err)
	}
	for _, h := range []string{"", "abc", strings.Repeat("AB", 32), strings.Repeat("zz", 32)} {
		if _, err := NewDesktopSignIn("x", h, t0); !errors.Is(err, ErrInvalidDesktopKeyHash) {
			t.Fatalf("hash %q: %v", h, err)
		}
	}
}

func TestValidDesktopSignInSecret(t *testing.T) {
	if !ValidDesktopSignInSecret("bky_signin_" + strings.Repeat("0f", 24)) {
		t.Fatal("valid secret refused")
	}
	for _, s := range []string{"", "bky_signin_abc", "bky_desk_" + strings.Repeat("0f", 24), "bky_signin_" + strings.Repeat("0F", 24)} {
		if ValidDesktopSignInSecret(s) {
			t.Fatalf("%q accepted", s)
		}
	}
}

func TestDesktopSignInStatus(t *testing.T) {
	s := newSignIn(t)
	if got := s.Status(t0); got != DesktopSignInPending {
		t.Fatalf("new: %s", got)
	}
	if got := s.Status(s.ExpiresAt); got != DesktopSignInExpired {
		t.Fatalf("at expiry: %s", got)
	}
	d, err := s.Approve(7, t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if d.MemberID != 7 || d.Name != "laptop" || d.KeyHash != keyHash || *s.ApprovedByMemberID != 7 {
		t.Fatalf("desktop %+v sign-in %+v", d, s)
	}
	if got := s.Status(t0.Add(2 * time.Minute)); got != DesktopSignInApproved {
		t.Fatalf("approved: %s", got)
	}
	// Paperclip's order: expiry is read before approval.
	if got := s.Status(s.ExpiresAt); got != DesktopSignInExpired {
		t.Fatalf("approved, then expired: %s", got)
	}
}

func TestDesktopSignInRefusesAllButPending(t *testing.T) {
	approved := newSignIn(t)
	if _, err := approved.Approve(1, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := approved.Approve(2, t0); !errors.Is(err, ErrDesktopSignInApproved) {
		t.Fatalf("approve twice: %v", err)
	}
	if err := approved.Cancel(t0); !errors.Is(err, ErrDesktopSignInApproved) {
		t.Fatalf("cancel approved: %v", err)
	}

	cancelled := newSignIn(t)
	if err := cancelled.Cancel(t0); err != nil {
		t.Fatal(err)
	}
	if _, err := cancelled.Approve(1, t0); !errors.Is(err, ErrDesktopSignInCancelled) {
		t.Fatalf("approve cancelled: %v", err)
	}

	expired := newSignIn(t)
	late := expired.ExpiresAt.Add(time.Second)
	if _, err := expired.Approve(1, late); !errors.Is(err, ErrDesktopSignInExpired) {
		t.Fatalf("approve expired: %v", err)
	}
	if err := expired.Cancel(late); !errors.Is(err, ErrDesktopSignInExpired) {
		t.Fatalf("cancel expired: %v", err)
	}
}

func TestDesktopCounts(t *testing.T) {
	d := Desktop{CreatedAt: t0}
	if !d.Counts(t0.Add(29 * 24 * time.Hour)) {
		t.Fatal("fresh desktop does not count")
	}
	if d.Counts(t0.Add(30 * 24 * time.Hour)) {
		t.Fatal("desktop unused for 30 days counts")
	}
	seen := t0.Add(20 * 24 * time.Hour)
	d.LastSeenAt = &seen
	if !d.Counts(t0.Add(45 * 24 * time.Hour)) {
		t.Fatal("desktop seen 25 days ago does not count")
	}
	d.SignOut(t0.Add(time.Hour))
	first := *d.RevokedAt
	d.SignOut(t0.Add(2 * time.Hour))
	if !d.RevokedAt.Equal(first) || d.Counts(t0.Add(2*time.Hour)) {
		t.Fatalf("signed out: %+v", d)
	}
}
