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

func (m *memStore) List(_ context.Context, guildID uint64) ([]domain.Server, error) {
	var out []domain.Server
	for i := uint64(1); i <= m.next; i++ {
		if s, ok := m.servers[i]; ok && (guildID == 0 || s.UsableBy(guildID)) {
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

func (m *memStore) NameTaken(_ context.Context, guildID uint64, name string, except uint64) (bool, error) {
	for _, s := range m.servers {
		if s.ID != except && strings.EqualFold(s.Name, name) && (guildID == 0 || s.UsableBy(guildID)) {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) AddressTaken(_ context.Context, guildID uint64, host string, port int, user string, except uint64) (bool, error) {
	for _, s := range m.servers {
		if s.ID != except && s.GuildID == guildID && s.Host == host && s.Port == port && s.User == user {
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

func fakeKey(comment string) (domain.PrivateKey, error) {
	return domain.PrivateKey{Public: "ssh-ed25519 AAAA " + comment, Private: "PRIVATE"}, nil
}

func TestEnsureLocalOnce(t *testing.T) {
	s := NewService(newMemStore(), fakeKey, nil)
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
	s := NewService(newMemStore(), fakeKey, nil)
	ctx := context.Background()
	srv, err := s.Add(ctx, 1, domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Key.Public != "ssh-ed25519 AAAA bakery@web" {
		t.Fatalf("key %q", srv.Key.Public)
	}
	var fe *domain.FieldError
	if _, err := s.Add(ctx, 1, domain.Input{Name: "WEB", Host: "10.0.0.2", User: "bakery"}); !errors.As(err, &fe) || fe.Field != "name" {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := s.Add(ctx, 1, domain.Input{Name: "web2", Host: "10.0.0.1", User: "bakery"}); !errors.As(err, &fe) || fe.Field != "host" {
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

func TestServersPerGuild(t *testing.T) {
	ctx := context.Background()
	s := NewService(newMemStore(), fakeKey, nil)
	if _, err := s.EnsureLocal(ctx); err != nil {
		t.Fatal(err)
	}
	a, err := s.Add(ctx, 1, domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})
	if err != nil {
		t.Fatal(err)
	}
	// Another Guild may use the same name and reach the same machine.
	b, err := s.Add(ctx, 2, domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})
	if err != nil {
		t.Fatalf("same name in another guild: %v", err)
	}
	var fe *domain.FieldError
	if _, err := s.Add(ctx, 2, domain.Input{Name: "LOCALHOST", Host: "10.0.0.3", User: "bakery"}); !errors.As(err, &fe) || fe.Field != "name" {
		t.Fatalf("the local server's name: %v", err)
	}
	list, err := s.List(ctx, 2)
	if err != nil || len(list) != 2 || list[0].Kind != domain.Local || list[1].ID != b.ID {
		t.Fatalf("guild 2 lists %+v %v", list, err)
	}
	if a.UsableBy(2) || !b.UsableBy(2) || !list[0].UsableBy(2) {
		t.Fatal("UsableBy")
	}
}

func TestReachAndDeleteInUse(t *testing.T) {
	ctx := context.Background()
	s := NewService(newMemStore(), fakeKey, fakeConnector{conn: healthy()})
	var forgotten []uint64
	s.Forget = func(id uint64) { forgotten = append(forgotten, id) }

	local, err := s.Reach(ctx, 0)
	if err != nil || local.Kind != domain.Local {
		t.Fatalf("%+v %v", local, err)
	}
	remote, _ := s.Add(ctx, 1, domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})
	if _, err := s.Reach(ctx, remote.ID); !errors.Is(err, ErrNotValidated) {
		t.Fatalf("unvalidated: %v", err)
	}
	if _, err := s.Reach(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := s.Validate(ctx, remote.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Reach(ctx, remote.ID); err != nil || got.ID != remote.ID {
		t.Fatalf("%+v %v", got, err)
	}

	used := true
	var asked []uint64
	s.OnDeleting(func(_ context.Context, id uint64) (bool, error) {
		asked = append(asked, id)
		return used, nil
	})
	if err := s.Delete(ctx, remote.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("in use: %v", err)
	}
	if _, err := s.ForgetHostKey(ctx, remote.ID); err != nil {
		t.Fatal(err)
	}
	used = false
	if err := s.Delete(ctx, remote.ID); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 2 || asked[0] != remote.ID {
		t.Fatalf("asked %v", asked)
	}
	if len(forgotten) != 2 || forgotten[0] != remote.ID || forgotten[1] != remote.ID {
		t.Fatalf("forgotten %v", forgotten)
	}
}
