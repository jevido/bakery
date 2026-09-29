// Package app holds the identity use cases: Setup, Login and finding the
// signed-in Owner.
package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

var (
	ErrOwnerExists    = errors.New("setup is already done")
	ErrBadCredentials = errors.New("email or password is wrong")
	ErrOwnerNotFound  = errors.New("owner not found")
)

// Owners stores the Owner.
type Owners interface {
	// Exists reports whether an Owner has been set up.
	Exists(ctx context.Context) (bool, error)
	// AddIfNone stores the Owner unless one exists already (ErrOwnerExists).
	// The check and the insert are one step, so racing Setups cannot both win.
	AddIfNone(ctx context.Context, owner domain.Owner) (domain.Owner, error)
	ByEmail(ctx context.Context, email string) (domain.Owner, bool, error)
	ByID(ctx context.Context, id uint64) (domain.Owner, bool, error)
}

// Hasher hashes and checks passwords.
type Hasher interface {
	Make(password string) (string, error)
	Check(password, hash string) bool
}

type Service struct {
	owners Owners
	hasher Hasher
}

func NewService(owners Owners, hasher Hasher) *Service {
	return &Service{owners: owners, hasher: hasher}
}

// SetupNeeded reports whether no Owner exists yet.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	exists, err := s.owners.Exists(ctx)
	return !exists, err
}

// SetupOwner creates the Owner, once.
func (s *Service) SetupOwner(ctx context.Context, name, email, password string) (domain.Owner, error) {
	owner, err := domain.NewOwner(name, email, password)
	if err != nil {
		return domain.Owner{}, err
	}
	if exists, err := s.owners.Exists(ctx); err != nil {
		return domain.Owner{}, err
	} else if exists {
		return domain.Owner{}, ErrOwnerExists
	}
	owner.PasswordHash, err = s.hasher.Make(password)
	if err != nil {
		return domain.Owner{}, err
	}
	return s.owners.AddIfNone(ctx, owner)
}

// Login checks the credentials. A wrong email and a wrong password give the
// same error.
func (s *Service) Login(ctx context.Context, email, password string) (domain.Owner, error) {
	owner, found, err := s.owners.ByEmail(ctx, domain.NormalizeEmail(email))
	if err != nil {
		return domain.Owner{}, err
	}
	if !found || !s.hasher.Check(password, owner.PasswordHash) {
		return domain.Owner{}, ErrBadCredentials
	}
	return owner, nil
}

// CurrentOwner returns the Owner a Session points at.
func (s *Service) CurrentOwner(ctx context.Context, id uint64) (domain.Owner, error) {
	owner, found, err := s.owners.ByID(ctx, id)
	if err != nil {
		return domain.Owner{}, err
	}
	if !found {
		return domain.Owner{}, ErrOwnerNotFound
	}
	return owner, nil
}
