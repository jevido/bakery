package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

var (
	ErrTwoFactorOn         = errors.New("two-factor authentication is already on")
	ErrTwoFactorOff        = errors.New("two-factor authentication is off")
	ErrTwoFactorNotPending = errors.New("start setting up two-factor authentication first")
)

// TwoFactorStatus is what a Member sees of their own Two-factor
// authentication.
type TwoFactorStatus struct {
	State             domain.TwoFactorState
	RecoveryCodesLeft int
}

// TwoFactorSetup is a new secret, to type or scan into an authenticator
// app.
type TwoFactorSetup struct {
	Secret     string
	OTPAuthURI string
}

func (s *Service) TwoFactorStatus(ctx context.Context, memberID uint64) (TwoFactorStatus, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return TwoFactorStatus{}, err
	}
	st := TwoFactorStatus{State: m.TwoFactor.State}
	if m.TwoFactor.On() {
		st.RecoveryCodesLeft, err = s.members.RecoveryCodesLeft(ctx, memberID)
	}
	return st, err
}

// StartTwoFactor makes a new secret, pending until ConfirmTwoFactor. Again
// while pending replaces the secret.
func (s *Service) StartTwoFactor(ctx context.Context, memberID uint64) (TwoFactorSetup, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return TwoFactorSetup{}, err
	}
	if m.TwoFactor.On() {
		return TwoFactorSetup{}, ErrTwoFactorOn
	}
	secret, err := domain.NewTOTPSecret(s.Random)
	if err != nil {
		return TwoFactorSetup{}, err
	}
	if err := s.members.SetPendingTwoFactor(ctx, memberID, secret); err != nil {
		return TwoFactorSetup{}, err
	}
	return TwoFactorSetup{Secret: domain.EncodeTOTPSecret(secret), OTPAuthURI: domain.OTPAuthURI(m.Email, secret)}, nil
}

// ConfirmTwoFactor switches the pending secret on with a code from the app
// and returns the Recovery codes, the only time they are shown.
func (s *Service) ConfirmTwoFactor(ctx context.Context, memberID uint64, code string) ([]string, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if m.TwoFactor.On() {
		return nil, ErrTwoFactorOn
	}
	if m.TwoFactor.State != domain.TwoFactorPending {
		return nil, ErrTwoFactorNotPending
	}
	step, err := domain.VerifyTOTP(m.TwoFactor.Secret, code, s.now(), m.TwoFactor.LastStep)
	if err != nil {
		return nil, err
	}
	codes, hashes, err := s.newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	return codes, s.members.EnableTwoFactor(ctx, memberID, step, s.now(), hashes)
}

// RegenerateRecoveryCodes replaces the Recovery codes, after a code from
// the app.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, memberID uint64, code string) ([]string, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if !m.TwoFactor.On() {
		return nil, ErrTwoFactorOff
	}
	if err := s.checkAuthenticatorCode(ctx, m, code); err != nil {
		return nil, err
	}
	codes, hashes, err := s.newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	return codes, s.members.ReplaceRecoveryCodes(ctx, memberID, hashes)
}

// DisableTwoFactor switches it off, with the password and either an
// Authenticator code or a Recovery code.
func (s *Service) DisableTwoFactor(ctx context.Context, memberID uint64, password, code, recoveryCode string) error {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return err
	}
	if !m.TwoFactor.On() {
		return ErrTwoFactorOff
	}
	if !s.hasher.Check(password, m.PasswordHash) {
		return ErrBadCredentials
	}
	if err := s.checkSecondFactor(ctx, m, code, recoveryCode); err != nil {
		return err
	}
	return s.members.ClearTwoFactor(ctx, memberID, nil)
}

// checkSecondFactor accepts an Authenticator code, or else a Recovery code,
// and uses it up.
func (s *Service) checkSecondFactor(ctx context.Context, m domain.Member, code, recoveryCode string) error {
	if code == "" && recoveryCode != "" {
		used, err := s.members.UseRecoveryCode(ctx, m.ID, hashSecret(domain.NormalizeRecoveryCode(recoveryCode)))
		if err != nil {
			return err
		}
		if !used {
			return domain.ErrWrongCode
		}
		return nil
	}
	return s.checkAuthenticatorCode(ctx, m, code)
}

func (s *Service) checkAuthenticatorCode(ctx context.Context, m domain.Member, code string) error {
	step, err := domain.VerifyTOTP(m.TwoFactor.Secret, code, s.now(), m.TwoFactor.LastStep)
	if err != nil {
		return err
	}
	advanced, err := s.members.AdvanceTwoFactorStep(ctx, m.ID, step)
	if err != nil {
		return err
	}
	if !advanced {
		return domain.ErrCodeUsed
	}
	return nil
}

func (s *Service) newRecoveryCodes() (codes, hashes []string, err error) {
	codes, err = domain.NewRecoveryCodes(s.Random)
	if err != nil {
		return nil, nil, err
	}
	hashes = make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = hashSecret(domain.NormalizeRecoveryCode(c))
	}
	return codes, hashes, nil
}
