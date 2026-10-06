package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// MembersByID lists the Members with these ids, for guilds' Members page.
func (s *Service) MembersByID(ctx context.Context, ids []uint64) ([]domain.Member, error) {
	return s.members.ByIDs(ctx, ids)
}

// ResetTwoFactor switches a locked-out Member's two-factor off and ends
// their Sessions. Who may do it (an admin of a Guild the Member is in,
// never for the Instance admin or themselves) is guilds' to decide.
func (s *Service) ResetTwoFactor(ctx context.Context, memberID uint64) error {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return err
	}
	return s.clearTwoFactor(ctx, m)
}

// ResetTwoFactorByEmail is ResetTwoFactor for whoever has a shell on the
// server: anyone, the Instance admin included.
func (s *Service) ResetTwoFactorByEmail(ctx context.Context, email string) error {
	m, found, err := s.members.ByEmail(ctx, domain.NormalizeEmail(email))
	if err != nil {
		return err
	}
	if !found {
		return ErrMemberNotFound
	}
	return s.clearTwoFactor(ctx, m)
}

func (s *Service) clearTwoFactor(ctx context.Context, m domain.Member) error {
	if !m.TwoFactor.On() && m.TwoFactor.State != domain.TwoFactorPending {
		return ErrTwoFactorOff
	}
	from := domain.SessionsValidFromNow(s.now())
	return s.members.ClearTwoFactor(ctx, m.ID, &from)
}
