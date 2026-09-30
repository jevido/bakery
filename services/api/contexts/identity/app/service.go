// Package app holds the identity use cases: Setup, Login and finding the
// Member a request comes from.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

var (
	ErrOwnerExists    = errors.New("setup is already done")
	ErrBadCredentials = errors.New("email or password is wrong")
	ErrMemberNotFound = errors.New("member not found")
)

// Members stores the Members.
type Members interface {
	// OwnerExists reports whether an Owner has been set up.
	OwnerExists(ctx context.Context) (bool, error)
	// AddOwnerIfNone stores the Owner unless one exists already
	// (ErrOwnerExists). The check and the insert are one step, so racing
	// Setups cannot both win.
	AddOwnerIfNone(ctx context.Context, owner domain.Member) (domain.Member, error)
	ByEmail(ctx context.Context, email string) (domain.Member, bool, error)
	ByID(ctx context.Context, id uint64) (domain.Member, bool, error)
	// All lists every Member, the Owner first, then by name.
	All(ctx context.Context) ([]domain.Member, error)
	SetRole(ctx context.Context, id uint64, role domain.Role) error
	// Remove deletes the Member, and with them their API tokens.
	Remove(ctx context.Context, id uint64) error
}

// Hasher hashes and checks passwords.
type Hasher interface {
	Make(password string) (string, error)
	Check(password, hash string) bool
}

type Service struct {
	members     Members
	invitations Invitations
	apiTokens   APITokens
	hasher      Hasher
	// Now is the clock; time.Now unless a test sets it.
	Now func() time.Time
}

func NewService(members Members, invitations Invitations, apiTokens APITokens, hasher Hasher) *Service {
	return &Service{members: members, invitations: invitations, apiTokens: apiTokens, hasher: hasher, Now: time.Now}
}

func (s *Service) now() time.Time { return s.Now() }

// SetupNeeded reports whether no Owner exists yet.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	exists, err := s.members.OwnerExists(ctx)
	return !exists, err
}

// SetupOwner creates the Owner, once.
func (s *Service) SetupOwner(ctx context.Context, name, email, password string) (domain.Member, error) {
	owner, err := domain.NewMember(name, email, password, domain.RoleOwner)
	if err != nil {
		return domain.Member{}, err
	}
	if exists, err := s.members.OwnerExists(ctx); err != nil {
		return domain.Member{}, err
	} else if exists {
		return domain.Member{}, ErrOwnerExists
	}
	owner.PasswordHash, err = s.hasher.Make(password)
	if err != nil {
		return domain.Member{}, err
	}
	return s.members.AddOwnerIfNone(ctx, owner)
}

// Login checks the credentials. A wrong email and a wrong password give the
// same error.
func (s *Service) Login(ctx context.Context, email, password string) (domain.Member, error) {
	m, found, err := s.members.ByEmail(ctx, domain.NormalizeEmail(email))
	if err != nil {
		return domain.Member{}, err
	}
	if !found || !s.hasher.Check(password, m.PasswordHash) {
		return domain.Member{}, ErrBadCredentials
	}
	return m, nil
}

// CurrentMember returns the Member a Session or API token points at; a
// removed Member is ErrMemberNotFound.
func (s *Service) CurrentMember(ctx context.Context, id uint64) (domain.Member, error) {
	m, found, err := s.members.ByID(ctx, id)
	if err != nil {
		return domain.Member{}, err
	}
	if !found {
		return domain.Member{}, ErrMemberNotFound
	}
	return m, nil
}
