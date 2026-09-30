package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// ChangeName renames the Member.
func (s *Service) ChangeName(ctx context.Context, memberID uint64, name string) (domain.Member, error) {
	name, err := domain.ValidateName(name)
	if err != nil {
		return domain.Member{}, err
	}
	if _, err := s.CurrentMember(ctx, memberID); err != nil {
		return domain.Member{}, err
	}
	if err := s.members.SetName(ctx, memberID, name); err != nil {
		return domain.Member{}, err
	}
	return s.CurrentMember(ctx, memberID)
}

// ChangePassword needs the current password, and ends every other Session:
// the caller hands the current browser a fresh one.
func (s *Service) ChangePassword(ctx context.Context, memberID uint64, current, next string) (domain.Member, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return domain.Member{}, err
	}
	if !s.hasher.Check(current, m.PasswordHash) {
		return domain.Member{}, ErrBadCredentials
	}
	if err := domain.ValidatePassword(next); err != nil {
		return domain.Member{}, err
	}
	hash, err := s.hasher.Make(next)
	if err != nil {
		return domain.Member{}, err
	}
	if err := s.members.SetPassword(ctx, memberID, hash, domain.SessionsValidFromNow(s.now())); err != nil {
		return domain.Member{}, err
	}
	return s.CurrentMember(ctx, memberID)
}

// SignOutOtherSessions ends every Session of the Member issued before now;
// the caller hands the current browser a fresh one.
func (s *Service) SignOutOtherSessions(ctx context.Context, memberID uint64) (domain.Member, error) {
	if _, err := s.CurrentMember(ctx, memberID); err != nil {
		return domain.Member{}, err
	}
	if err := s.members.SetSessionsValidFrom(ctx, memberID, domain.SessionsValidFromNow(s.now())); err != nil {
		return domain.Member{}, err
	}
	return s.CurrentMember(ctx, memberID)
}
