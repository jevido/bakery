package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

func TestAccount(t *testing.T) {
	ctx := context.Background()
	s, now, m := twoFactorService(t)

	if _, err := s.ChangeName(ctx, m.ID, "  "); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("empty name: %v", err)
	}
	if got, err := s.ChangeName(ctx, m.ID, " Ada L. "); err != nil || got.Name != "Ada L." {
		t.Fatalf("rename: %+v %v", got, err)
	}
	if _, err := s.ChangePassword(ctx, m.ID, "not the password", "a much longer password"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("wrong current: %v", err)
	}
	if _, err := s.ChangePassword(ctx, m.ID, "correct horse battery", "short"); !errors.Is(err, domain.ErrPasswordTooShort) {
		t.Fatalf("short: %v", err)
	}
	*now = now.Add(1500 * time.Millisecond)
	got, err := s.ChangePassword(ctx, m.ID, "correct horse battery", "a much longer password")
	if err != nil {
		t.Fatal(err)
	}
	if !got.SessionsValidFrom.Equal(now.Truncate(time.Second)) {
		t.Fatalf("sessions valid from %v", got.SessionsValidFrom)
	}
	if _, err := s.Login(ctx, "ada@example.com", "correct horse battery"); !errors.Is(err, ErrBadCredentials) {
		t.Fatal("old password still signs in")
	}
	if _, err := s.Login(ctx, "ada@example.com", "a much longer password"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	*now = now.Add(time.Minute)
	got, err = s.SignOutOtherSessions(ctx, m.ID)
	if err != nil || !got.SessionsValidFrom.Equal(now.Truncate(time.Second)) {
		t.Fatalf("sign out others: %v %v", got.SessionsValidFrom, err)
	}
}
