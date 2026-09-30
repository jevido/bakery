package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// AllMembers lists every Member, the Owner first.
func (s *Service) AllMembers(ctx context.Context) ([]domain.Member, error) {
	return s.members.All(ctx)
}

// ChangeRole gives a Member another Role, by the domain's rules.
func (s *Service) ChangeRole(ctx context.Context, actorID, memberID uint64, role domain.Role) (domain.Member, error) {
	actor, target, err := s.actorAndTarget(ctx, actorID, memberID)
	if err != nil {
		return domain.Member{}, err
	}
	if err := domain.CanGrant(actor, role); err != nil {
		return domain.Member{}, err
	}
	if err := domain.CanManage(actor, target); err != nil {
		return domain.Member{}, err
	}
	if err := s.members.SetRole(ctx, target.ID, role); err != nil {
		return domain.Member{}, err
	}
	target.Role = role
	return target, nil
}

// RemoveMember removes a Member; their Sessions and API tokens stop working
// on their next request.
func (s *Service) RemoveMember(ctx context.Context, actorID, memberID uint64) error {
	actor, target, err := s.actorAndTarget(ctx, actorID, memberID)
	if err != nil {
		return err
	}
	if err := domain.CanManage(actor, target); err != nil {
		return err
	}
	return s.members.Remove(ctx, target.ID)
}

func (s *Service) actorAndTarget(ctx context.Context, actorID, memberID uint64) (domain.Member, domain.Member, error) {
	actor, err := s.CurrentMember(ctx, actorID)
	if err != nil {
		return domain.Member{}, domain.Member{}, err
	}
	target, err := s.CurrentMember(ctx, memberID)
	return actor, target, err
}

// ResetTwoFactor switches a locked-out Member's two-factor off and ends
// their Sessions, by the same rules as changing their Role: never the
// Owner's, never your own.
func (s *Service) ResetTwoFactor(ctx context.Context, actorID, memberID uint64) error {
	actor, target, err := s.actorAndTarget(ctx, actorID, memberID)
	if err != nil {
		return err
	}
	if err := domain.CanManage(actor, target); err != nil {
		return err
	}
	return s.clearTwoFactor(ctx, target)
}

// ResetTwoFactorByEmail is ResetTwoFactor for whoever has a shell on the
// server: anyone, the Owner included.
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
