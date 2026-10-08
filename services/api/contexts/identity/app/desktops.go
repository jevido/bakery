package app

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// ErrDesktopSignInNotFound answers an unknown id and a wrong secret alike,
// so a sign-in cannot be found without its secret.
var ErrDesktopSignInNotFound = errors.New("desktop sign-in not found")

// DesktopSignIns stores the Desktop sign-ins and the Desktops they make.
type DesktopSignIns interface {
	// Add stores a new sign-in with the hash of its secret.
	Add(ctx context.Context, s domain.DesktopSignIn, secretHash string) (domain.DesktopSignIn, error)
	// ByID answers the sign-in and its secret hash.
	ByID(ctx context.Context, id uint64) (domain.DesktopSignIn, string, bool, error)
	// Change locks the sign-in's row and lets change act on it. A Desktop
	// change answers is stored with it and the sign-in points at it; nothing
	// is stored when change fails. ErrDesktopSignInNotFound when there is
	// no such sign-in.
	Change(ctx context.Context, id uint64, change func(s *domain.DesktopSignIn, secretHash string) (*domain.Desktop, error)) (domain.DesktopSignIn, error)
}

// DesktopSignInView is a sign-in as its secret's holder reads it.
type DesktopSignInView struct {
	SignIn     domain.DesktopSignIn
	Status     domain.DesktopSignInStatus
	ApprovedBy *domain.Member
}

func secretMatches(stored, secretValue string) bool {
	return subtle.ConstantTimeCompare([]byte(stored), []byte(secret.Hash(secretValue))) == 1
}

// StartDesktopSignIn stores the Desktop app's request to be approved. The
// app minted secretValue and the Desktop key; the key only arrives hashed.
func (s *Service) StartDesktopSignIn(ctx context.Context, clientName, secretValue, keyHash string) (domain.DesktopSignIn, error) {
	if !domain.ValidDesktopSignInSecret(secretValue) {
		return domain.DesktopSignIn{}, domain.ErrInvalidDesktopSignInKey
	}
	in, err := domain.NewDesktopSignIn(clientName, keyHash, s.now())
	if err != nil {
		return domain.DesktopSignIn{}, err
	}
	return s.desktopSignIns.Add(ctx, in, secret.Hash(secretValue))
}

// DescribeDesktopSignIn reads a sign-in for whoever holds its secret.
func (s *Service) DescribeDesktopSignIn(ctx context.Context, id uint64, secretValue string) (DesktopSignInView, error) {
	in, hash, found, err := s.desktopSignIns.ByID(ctx, id)
	if err != nil {
		return DesktopSignInView{}, err
	}
	if !found || !secretMatches(hash, secretValue) {
		return DesktopSignInView{}, ErrDesktopSignInNotFound
	}
	v := DesktopSignInView{SignIn: in, Status: in.Status(s.now())}
	if in.ApprovedByMemberID != nil {
		m, found, err := s.members.ByID(ctx, *in.ApprovedByMemberID)
		if err != nil {
			return DesktopSignInView{}, err
		}
		if found {
			v.ApprovedBy = &m
		}
	}
	return v, nil
}

// ApproveDesktopSignIn approves a pending sign-in for the Member, making
// their Desktop. Approving an approved one answers it unchanged.
func (s *Service) ApproveDesktopSignIn(ctx context.Context, id uint64, secretValue string, memberID uint64) (domain.DesktopSignIn, error) {
	if _, err := s.CurrentMember(ctx, memberID); err != nil {
		return domain.DesktopSignIn{}, err
	}
	now := s.now()
	return s.desktopSignIns.Change(ctx, id, func(in *domain.DesktopSignIn, hash string) (*domain.Desktop, error) {
		if !secretMatches(hash, secretValue) {
			return nil, ErrDesktopSignInNotFound
		}
		if in.Status(now) == domain.DesktopSignInApproved {
			return nil, nil
		}
		d, err := in.Approve(memberID, now)
		if err != nil {
			return nil, err
		}
		return &d, nil
	})
}

// CancelDesktopSignIn ends a pending sign-in for whoever holds its secret.
func (s *Service) CancelDesktopSignIn(ctx context.Context, id uint64, secretValue string) (domain.DesktopSignIn, error) {
	now := s.now()
	return s.desktopSignIns.Change(ctx, id, func(in *domain.DesktopSignIn, hash string) (*domain.Desktop, error) {
		if !secretMatches(hash, secretValue) {
			return nil, ErrDesktopSignInNotFound
		}
		return nil, in.Cancel(now)
	})
}

// DesktopSignInStatus is the sign-in's status at the service's clock.
func (s *Service) DesktopSignInStatus(in domain.DesktopSignIn) domain.DesktopSignInStatus {
	return in.Status(s.now())
}
