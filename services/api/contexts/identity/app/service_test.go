package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type memMembers struct {
	mu      sync.Mutex
	members []domain.Member
}

func (m *memMembers) OwnerExists(context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.members {
		if x.Role == domain.RoleOwner {
			return true, nil
		}
	}
	return false, nil
}

func (m *memMembers) AddOwnerIfNone(ctx context.Context, o domain.Member) (domain.Member, error) {
	if exists, _ := m.OwnerExists(ctx); exists {
		return domain.Member{}, ErrOwnerExists
	}
	return m.add(o), nil
}

func (m *memMembers) add(x domain.Member) domain.Member {
	m.mu.Lock()
	defer m.mu.Unlock()
	x.ID = uint64(len(m.members) + 1)
	m.members = append(m.members, x)
	return x
}

func (m *memMembers) find(match func(domain.Member) bool) (domain.Member, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.members {
		if match(x) {
			return x, true, nil
		}
	}
	return domain.Member{}, false, nil
}

func (m *memMembers) ByEmail(_ context.Context, email string) (domain.Member, bool, error) {
	return m.find(func(x domain.Member) bool { return x.Email == email })
}

func (m *memMembers) ByID(_ context.Context, id uint64) (domain.Member, bool, error) {
	return m.find(func(x domain.Member) bool { return x.ID == id })
}

func (m *memMembers) All(context.Context) ([]domain.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Member(nil), m.members...), nil
}

func (m *memMembers) SetRole(_ context.Context, id uint64, role domain.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.members {
		if m.members[i].ID == id {
			m.members[i].Role = role
		}
	}
	return nil
}

func (m *memMembers) Remove(_ context.Context, id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.members {
		if m.members[i].ID == id {
			m.members = append(m.members[:i], m.members[i+1:]...)
			return nil
		}
	}
	return nil
}

func newTestService() *Service {
	members := &memMembers{}
	return NewService(members, &memInvitations{members: members}, &memAPITokens{}, plainHasher{})
}

type plainHasher struct{}

func (plainHasher) Make(p string) (string, error) { return "h:" + p, nil }
func (plainHasher) Check(p, h string) bool        { return h == "h:"+p }

func TestSetupOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s := newTestService()

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
	s := newTestService()
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
