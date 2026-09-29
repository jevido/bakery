package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type memOwners struct {
	mu    sync.Mutex
	owner *domain.Owner
}

func (m *memOwners) Exists(context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.owner != nil, nil
}

func (m *memOwners) AddIfNone(_ context.Context, o domain.Owner) (domain.Owner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owner != nil {
		return domain.Owner{}, ErrOwnerExists
	}
	o.ID = 1
	m.owner = &o
	return o, nil
}

func (m *memOwners) ByEmail(_ context.Context, email string) (domain.Owner, bool, error) {
	if m.owner == nil || m.owner.Email != email {
		return domain.Owner{}, false, nil
	}
	return *m.owner, true, nil
}

func (m *memOwners) ByID(_ context.Context, id uint64) (domain.Owner, bool, error) {
	if m.owner == nil || m.owner.ID != id {
		return domain.Owner{}, false, nil
	}
	return *m.owner, true, nil
}

type plainHasher struct{}

func (plainHasher) Make(p string) (string, error) { return "h:" + p, nil }
func (plainHasher) Check(p, h string) bool        { return h == "h:"+p }

func TestSetupOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s := NewService(&memOwners{}, plainHasher{})

	if needed, _ := s.SetupNeeded(ctx); !needed {
		t.Fatal("setup should be needed on a fresh install")
	}
	if _, err := s.SetupOwner(ctx, "Ada", "ada@example.com", "correct horse"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if _, err := s.SetupOwner(ctx, "Bram", "bram@example.com", "correct horse"); !errors.Is(err, ErrOwnerExists) {
		t.Fatalf("second setup: err = %v, want ErrOwnerExists", err)
	}
	if needed, _ := s.SetupNeeded(ctx); needed {
		t.Fatal("setup should not be needed after it ran")
	}
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	s := NewService(&memOwners{}, plainHasher{})
	if _, err := s.SetupOwner(ctx, "Ada", "ada@example.com", "correct horse"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(ctx, "ADA@example.com", "correct horse"); err != nil {
		t.Fatalf("right password: %v", err)
	}
	if _, err := s.Login(ctx, "ada@example.com", "wrong password"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("wrong password: err = %v", err)
	}
	if _, err := s.Login(ctx, "nobody@example.com", "correct horse"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("unknown email: err = %v", err)
	}
}
