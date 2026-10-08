package app

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// ErrDesktopSignInNotFound answers an unknown id and a wrong secret alike,
// so a sign-in cannot be found without its secret.
var ErrDesktopSignInNotFound = errors.New("desktop sign-in not found")

// ErrInvalidDesktopKey answers an unknown Desktop key and one that no
// longer counts alike.
var ErrInvalidDesktopKey = errors.New("invalid desktop key")

// ErrDesktopNotFound answers an id that is not one of the Member's Desktops.
var ErrDesktopNotFound = errors.New("desktop not found")

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
	// DesktopByKeyHash is the Desktop holding the key with this hash.
	DesktopByKeyHash(ctx context.Context, keyHash string) (domain.Desktop, bool, error)
	// DesktopsOf lists the Member's Desktops, newest first, signed out ones
	// included.
	DesktopsOf(ctx context.Context, memberID uint64) ([]domain.Desktop, error)
	// DesktopNames names the Desktops among ids, signed out ones included.
	DesktopNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// TouchDesktop records that the Desktop was used at.
	TouchDesktop(ctx context.Context, id uint64, at time.Time) error
	// SignOutDesktop signs the Member's Desktop out at; false when the
	// Member has no Desktop with this id.
	SignOutDesktop(ctx context.Context, id, memberID uint64, at time.Time) (bool, error)
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

// AuthenticateDesktop finds the Member and Desktop a Desktop key belongs to.
// The key stops counting when the Desktop is signed out or idle (Counts),
// when its Member is gone, and when their Sessions valid from has moved
// past the Desktop's creation. Last seen is written at most once a minute.
func (s *Service) AuthenticateDesktop(ctx context.Context, key string) (domain.Member, domain.Desktop, error) {
	if !strings.HasPrefix(key, domain.DesktopKeyPrefix) {
		return domain.Member{}, domain.Desktop{}, ErrInvalidDesktopKey
	}
	d, found, err := s.desktopSignIns.DesktopByKeyHash(ctx, secret.Hash(key))
	if err != nil {
		return domain.Member{}, domain.Desktop{}, err
	}
	now := s.now()
	if !found || !d.Counts(now) {
		return domain.Member{}, domain.Desktop{}, ErrInvalidDesktopKey
	}
	m, err := s.CurrentMember(ctx, d.MemberID)
	if errors.Is(err, ErrMemberNotFound) {
		return domain.Member{}, domain.Desktop{}, ErrInvalidDesktopKey
	}
	if err != nil {
		return domain.Member{}, domain.Desktop{}, err
	}
	if !m.SessionCounts(d.CreatedAt) {
		return domain.Member{}, domain.Desktop{}, ErrInvalidDesktopKey
	}
	if d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) >= touchEvery {
		if err := s.desktopSignIns.TouchDesktop(ctx, d.ID, now); err != nil {
			return domain.Member{}, domain.Desktop{}, err
		}
		d.LastSeenAt = &now
	}
	return m, d, nil
}

// SeeDesktop records that the Desktop is still there, for a stream it
// keeps open longer than one request.
func (s *Service) SeeDesktop(ctx context.Context, id uint64) error {
	return s.desktopSignIns.TouchDesktop(ctx, id, s.now())
}

// DesktopNames names the Desktops among ids, signed out ones included.
func (s *Service) DesktopNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	if len(ids) == 0 {
		return map[uint64]string{}, nil
	}
	return s.desktopSignIns.DesktopNames(ctx, ids)
}

// Desktops lists the Member's Desktops that are still signed in, newest
// first. One whose key no longer counts (idle, or ended with the Sessions)
// is left out as signed out.
func (s *Service) Desktops(ctx context.Context, memberID uint64) ([]domain.Desktop, error) {
	m, err := s.CurrentMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	all, err := s.desktopSignIns.DesktopsOf(ctx, memberID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := []domain.Desktop{}
	for _, d := range all {
		if d.Counts(now) && m.SessionCounts(d.CreatedAt) {
			out = append(out, d)
		}
	}
	return out, nil
}

// SignOutDesktop signs out one of the Member's Desktops.
func (s *Service) SignOutDesktop(ctx context.Context, memberID, id uint64) error {
	found, err := s.desktopSignIns.SignOutDesktop(ctx, id, memberID, s.now())
	if err != nil {
		return err
	}
	if !found {
		return ErrDesktopNotFound
	}
	return nil
}
