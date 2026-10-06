package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type memStore struct {
	mu          sync.Mutex
	guilds      []domain.Guild
	memberships []domain.Membership
}

func (m *memStore) ByID(_ context.Context, id uint64) (domain.Guild, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, g := range m.guilds {
		if g.ID == id {
			return g, true, nil
		}
	}
	return domain.Guild{}, false, nil
}

func (m *memStore) All(context.Context) ([]domain.Guild, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Guild(nil), m.guilds...), nil
}

func (m *memStore) Create(_ context.Context, g domain.Guild, adminID uint64) (domain.Guild, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.create(g, adminID), nil
}

func (m *memStore) create(g domain.Guild, adminID uint64) domain.Guild {
	g.ID = uint64(len(m.guilds) + 1)
	m.guilds = append(m.guilds, g)
	m.memberships = append(m.memberships, domain.Membership{ID: uint64(len(m.memberships) + 1), GuildID: g.ID, MemberID: adminID, Role: domain.RoleAdmin})
	return g
}

func (m *memStore) CreateFirstIfNone(_ context.Context, g domain.Guild, adminID uint64) (domain.Guild, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.guilds) > 0 {
		return domain.Guild{}, false, nil
	}
	return m.create(g, adminID), true, nil
}

func (m *memStore) ListForMember(_ context.Context, memberID uint64) ([]domain.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Membership
	for _, x := range m.memberships {
		if x.MemberID == memberID {
			out = append(out, x)
		}
	}
	return out, nil
}

func (m *memStore) RoleOf(_ context.Context, guildID, memberID uint64) (domain.Role, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.memberships {
		if x.GuildID == guildID && x.MemberID == memberID {
			return x.Role, true, nil
		}
	}
	return "", false, nil
}

func newTestService() (*Service, *memStore) {
	m := &memStore{}
	return NewService(m, m), m
}

func TestMakeFirstGuildOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s, m := newTestService()
	if err := s.MakeFirstGuild(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.MakeFirstGuild(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if len(m.guilds) != 1 || m.guilds[0].Name != "Default" {
		t.Fatalf("guilds = %+v, want one Default", m.guilds)
	}
	if r, ok, _ := s.RoleOf(ctx, m.guilds[0].ID, 1); !ok || r != domain.RoleAdmin {
		t.Errorf("Instance admin's Role = %q, %v; want admin", r, ok)
	}
	if _, ok, _ := s.RoleOf(ctx, m.guilds[0].ID, 2); ok {
		t.Error("the second call made a Membership")
	}
}

func TestCreateGuildMakesTheCreatorAdmin(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService()
	if _, err := s.CreateGuild(ctx, " ", "", 7); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("empty name: err = %v", err)
	}
	g, err := s.CreateGuild(ctx, " Bakers ", "bread", 7)
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "Bakers" {
		t.Errorf("name = %q", g.Name)
	}
	ms, _ := s.GuildsOf(ctx, 7)
	if len(ms) != 1 || ms[0].GuildID != g.ID || ms[0].Role != domain.RoleAdmin {
		t.Errorf("memberships = %+v", ms)
	}
}
