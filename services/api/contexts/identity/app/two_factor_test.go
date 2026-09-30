package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// twoFactorService is a test Service at a fixed time with one Member.
func twoFactorService(t *testing.T) (*Service, *time.Time, domain.Member) {
	t.Helper()
	s := newTestService()
	now := time.Unix(1_800_000_000, 0)
	s.Now = func() time.Time { return now }
	m, err := s.SetupOwner(context.Background(), "Ada", "ada@example.com", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	return s, &now, m
}

func secretOf(t *testing.T, s *Service, id uint64) []byte {
	t.Helper()
	m, err := s.CurrentMember(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return m.TwoFactor.Secret
}

func codeAt(t *testing.T, s *Service, id uint64, now time.Time) string {
	return domain.TOTPCode(secretOf(t, s, id), domain.TOTPStep(now))
}

func TestTwoFactorEnrolment(t *testing.T) {
	ctx := context.Background()
	s, now, m := twoFactorService(t)

	if _, err := s.ConfirmTwoFactor(ctx, m.ID, "123456"); !errors.Is(err, ErrTwoFactorNotPending) {
		t.Fatalf("confirm before start: %v", err)
	}
	setup, err := s.StartTwoFactor(ctx, m.ID)
	if err != nil || setup.Secret == "" || setup.OTPAuthURI == "" {
		t.Fatalf("start: %+v %v", setup, err)
	}
	if st, _ := s.TwoFactorStatus(ctx, m.ID); st.State != domain.TwoFactorPending {
		t.Fatalf("state after start: %s", st.State)
	}
	if _, err := s.ConfirmTwoFactor(ctx, m.ID, "000000"); !errors.Is(err, domain.ErrWrongCode) {
		t.Fatalf("wrong code: %v", err)
	}
	code := codeAt(t, s, m.ID, *now)
	codes, err := s.ConfirmTwoFactor(ctx, m.ID, code)
	if err != nil || len(codes) != domain.RecoveryCodeCount {
		t.Fatalf("confirm: %d codes, %v", len(codes), err)
	}
	st, _ := s.TwoFactorStatus(ctx, m.ID)
	if st.State != domain.TwoFactorOn || st.RecoveryCodesLeft != 10 {
		t.Fatalf("status: %+v", st)
	}
	if _, err := s.StartTwoFactor(ctx, m.ID); !errors.Is(err, ErrTwoFactorOn) {
		t.Fatalf("start while on: %v", err)
	}

	// The confirming code is used up.
	if _, err := s.RegenerateRecoveryCodes(ctx, m.ID, code); !errors.Is(err, domain.ErrCodeUsed) {
		t.Fatalf("reused code: %v", err)
	}
	*now = now.Add(30 * time.Second)
	fresh, err := s.RegenerateRecoveryCodes(ctx, m.ID, codeAt(t, s, m.ID, *now))
	if err != nil {
		t.Fatal(err)
	}
	// Old Recovery codes stop working, new ones work once.
	if err := s.DisableTwoFactor(ctx, m.ID, "correct horse battery", "", codes[0]); !errors.Is(err, domain.ErrWrongCode) {
		t.Fatalf("old recovery code: %v", err)
	}
	if err := s.DisableTwoFactor(ctx, m.ID, "wrong password here", "", fresh[0]); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if err := s.DisableTwoFactor(ctx, m.ID, "correct horse battery", "", fresh[0]); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if st, _ := s.TwoFactorStatus(ctx, m.ID); st.State != domain.TwoFactorOff {
		t.Fatalf("state after disable: %s", st.State)
	}
	if err := s.DisableTwoFactor(ctx, m.ID, "correct horse battery", "", fresh[1]); !errors.Is(err, ErrTwoFactorOff) {
		t.Fatalf("disable twice: %v", err)
	}
}

func TestLoginTwoFactor(t *testing.T) {
	ctx := context.Background()
	s, now, m := twoFactorService(t)

	if _, err := s.LoginTwoFactor(ctx, m.ID, "123456", ""); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("two-factor off: %v", err)
	}
	if _, err := s.StartTwoFactor(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	codes, err := s.ConfirmTwoFactor(ctx, m.ID, codeAt(t, s, m.ID, *now))
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(30 * time.Second)
	code := codeAt(t, s, m.ID, *now)
	if got, err := s.LoginTwoFactor(ctx, m.ID, code, ""); err != nil || got.ID != m.ID {
		t.Fatalf("right code: %v", err)
	}
	if _, err := s.LoginTwoFactor(ctx, m.ID, code, ""); !errors.Is(err, domain.ErrCodeUsed) {
		t.Fatalf("same code again: %v", err)
	}
	if _, err := s.LoginTwoFactor(ctx, m.ID, "", codes[3]); err != nil {
		t.Fatalf("recovery code: %v", err)
	}
	if _, err := s.LoginTwoFactor(ctx, m.ID, "", codes[3]); !errors.Is(err, domain.ErrWrongCode) {
		t.Fatalf("recovery code twice: %v", err)
	}
	if left, _ := s.TwoFactorStatus(ctx, m.ID); left.RecoveryCodesLeft != 9 {
		t.Fatalf("left: %d", left.RecoveryCodesLeft)
	}
	if _, err := s.LoginTwoFactor(ctx, 999, code, ""); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("unknown member: %v", err)
	}
}

func TestResetTwoFactor(t *testing.T) {
	ctx := context.Background()
	s, now, owner := twoFactorService(t)
	join := func(email string, role domain.Role) domain.Member {
		_, token, err := s.Invite(ctx, owner.ID, email, role)
		if err != nil {
			t.Fatal(err)
		}
		m, err := s.AcceptInvitation(ctx, token, "x", "correct horse battery")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	admin := join("admin@example.com", domain.RoleAdmin)
	dev := join("dev@example.com", domain.RoleMember)
	enrol := func(id uint64) {
		if _, err := s.StartTwoFactor(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ConfirmTwoFactor(ctx, id, codeAt(t, s, id, *now)); err != nil {
			t.Fatal(err)
		}
	}
	enrol(owner.ID)
	enrol(admin.ID)
	enrol(dev.ID)

	if err := s.ResetTwoFactor(ctx, dev.ID, admin.ID); !errors.Is(err, domain.ErrNotAdmin) {
		t.Errorf("member resets admin: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, admin.ID, owner.ID); !errors.Is(err, domain.ErrOwnerIsFixed) {
		t.Errorf("admin resets owner: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, admin.ID, admin.ID); !errors.Is(err, domain.ErrSelf) {
		t.Errorf("admin resets self: %v", err)
	}
	*now = now.Add(2 * time.Second)
	if err := s.ResetTwoFactor(ctx, admin.ID, dev.ID); err != nil {
		t.Fatalf("admin resets member: %v", err)
	}
	m, _ := s.CurrentMember(ctx, dev.ID)
	if m.TwoFactor.On() || !m.SessionsValidFrom.Equal(now.Truncate(time.Second)) {
		t.Errorf("after reset: %+v", m)
	}
	if err := s.ResetTwoFactor(ctx, admin.ID, dev.ID); !errors.Is(err, ErrTwoFactorOff) {
		t.Errorf("reset twice: %v", err)
	}
	if err := s.ResetTwoFactorByEmail(ctx, " ADA@example.com"); err != nil {
		t.Errorf("owner from the server: %v", err)
	}
	if err := s.ResetTwoFactorByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrMemberNotFound) {
		t.Errorf("unknown email: %v", err)
	}
}
