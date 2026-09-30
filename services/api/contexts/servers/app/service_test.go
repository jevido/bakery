package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// memStore is an in-memory Store.
type memStore struct {
	servers map[uint64]domain.Server
	next    uint64
}

func newMemStore() *memStore { return &memStore{servers: map[uint64]domain.Server{}} }

func (m *memStore) Create(_ context.Context, s domain.Server) (domain.Server, error) {
	m.next++
	s.ID = m.next
	m.servers[s.ID] = s
	return s, nil
}

func (m *memStore) Get(_ context.Context, id uint64) (domain.Server, bool, error) {
	s, ok := m.servers[id]
	return s, ok, nil
}

func (m *memStore) List(context.Context) ([]domain.Server, error) {
	var out []domain.Server
	for i := uint64(1); i <= m.next; i++ {
		if s, ok := m.servers[i]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memStore) Save(_ context.Context, s domain.Server) error {
	m.servers[s.ID] = s
	return nil
}

func (m *memStore) Delete(_ context.Context, id uint64) error {
	delete(m.servers, id)
	return nil
}

func (m *memStore) NameTaken(_ context.Context, name string, except uint64) (bool, error) {
	for _, s := range m.servers {
		if s.ID != except && strings.EqualFold(s.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) AddressTaken(_ context.Context, host string, port int, user string, except uint64) (bool, error) {
	for _, s := range m.servers {
		if s.ID != except && s.Host == host && s.Port == port && s.User == user {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) Local(context.Context) (domain.Server, bool, error) {
	for _, s := range m.servers {
		if s.Kind == domain.Local {
			return s, true, nil
		}
	}
	return domain.Server{}, false, nil
}

func fakeKey(comment string) (domain.ServerKey, error) {
	return domain.ServerKey{Public: "ssh-ed25519 AAAA " + comment, Private: "PRIVATE"}, nil
}

func TestEnsureLocalOnce(t *testing.T) {
	s := NewService(newMemStore(), fakeKey)
	ctx := context.Background()
	a, err := s.EnsureLocal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.EnsureLocal(ctx)
	if a.ID != b.ID || a.Kind != domain.Local {
		t.Fatalf("%+v %+v", a, b)
	}
	if err := s.Delete(ctx, a.ID); !errors.Is(err, domain.ErrLocalServer) {
		t.Fatalf("delete local: %v", err)
	}
}

func TestAddUnique(t *testing.T) {
	s := NewService(newMemStore(), fakeKey)
	ctx := context.Background()
	srv, err := s.Add(ctx, domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Key.Public != "ssh-ed25519 AAAA bakery@web" {
		t.Fatalf("key %q", srv.Key.Public)
	}
	var fe *domain.FieldError
	if _, err := s.Add(ctx, domain.Input{Name: "WEB", Host: "10.0.0.2", User: "bakery"}); !errors.As(err, &fe) || fe.Field != "name" {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := s.Add(ctx, domain.Input{Name: "web2", Host: "10.0.0.1", User: "bakery"}); !errors.As(err, &fe) || fe.Field != "host" {
		t.Fatalf("duplicate address: %v", err)
	}
	if _, err := s.Edit(ctx, srv.ID, domain.Input{Name: "web", Host: "10.0.0.1", Port: 2222, User: "bakery"}); err != nil {
		t.Fatalf("edit own: %v", err)
	}
	if err := s.Delete(ctx, srv.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, srv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
}
