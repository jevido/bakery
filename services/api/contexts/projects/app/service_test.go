package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

// fakeStore implements only what CreateApplication and UpdateApplication
// touch; anything else panics through the nil embedded Store.
type fakeStore struct {
	Store
	app domain.Application
}

func (f *fakeStore) Environment(context.Context, uint64) (domain.Environment, bool, error) {
	return domain.Environment{ID: 1, ProjectID: 1, Name: domain.DefaultEnvironment}, true, nil
}
func (f *fakeStore) SlugTaken(context.Context, string) (bool, error) { return false, nil }
func (f *fakeStore) DomainsTaken(_ context.Context, domains []string, _ uint64) ([]string, error) {
	var taken []string
	for _, d := range domains {
		if d == "taken.example.com" {
			taken = append(taken, d)
		}
	}
	return taken, nil
}
func (f *fakeStore) CreateApplication(_ context.Context, a domain.Application) (domain.Application, error) {
	a.ID = 1
	f.app = a
	return a, nil
}
func (f *fakeStore) Application(context.Context, uint64) (domain.Application, bool, error) {
	return f.app, f.app.ID != 0, nil
}
func (f *fakeStore) UpdateApplication(_ context.Context, a domain.Application) error {
	f.app = a
	return nil
}

func TestDashboardDomainIsReserved(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, fakeKey, "example.com", "bakery.example.com")
	in := domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80}

	in.Domains = []string{"web.example.com", "Bakery.Example.com"}
	_, err := s.CreateApplication(ctx, 1, in)
	var fe *domain.FieldError
	if !errors.As(err, &fe) || fe.Field != "domains" {
		t.Fatalf("create with the dashboard domain: want a domain FieldError, got %v", err)
	}

	in.Domains = nil
	a, err := s.CreateApplication(ctx, 1, in)
	if err != nil || len(a.Domains) != 1 || a.Domains[0] != "web.example.com" {
		t.Fatalf("create with the default domain: %v, %q", err, a.Domains)
	}

	in.Domains = []string{"bakery.example.com"}
	if _, err := s.UpdateApplication(ctx, a.ID, in); !errors.As(err, &fe) || fe.Field != "domains" {
		t.Fatalf("update to the dashboard domain: want a domain FieldError, got %v", err)
	}
}

var keys int

func fakeKey(comment string) (domain.DeployKey, error) {
	keys++
	return domain.DeployKey{Public: fmt.Sprintf("ssh-ed25519 KEY%d %s", keys, comment), Private: "PRIVATE"}, nil
}

func TestSSHSourceAlwaysHasADeployKey(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	s := NewService(store, fakeKey, "example.com", "")

	a, err := s.CreateApplication(ctx, 1, domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80})
	if err != nil || a.DeployKey != (domain.DeployKey{}) {
		t.Fatalf("https source: %v, key %+v", err, a.DeployKey)
	}
	if _, err := s.RegenerateDeployKey(ctx, a.ID); err == nil {
		t.Fatal("regenerating the key of an https source must fail")
	}

	in := domain.ApplicationInput{Name: "web", GitURL: "git@example.com:me/r.git", Port: 80}
	a, err = s.UpdateApplication(ctx, a.ID, in)
	if err != nil || !strings.HasSuffix(a.DeployKey.Public, " bakery-web") || a.DeployKey.Private == "" {
		t.Fatalf("ssh source: %v, key %+v", err, a.DeployKey)
	}
	first := a.DeployKey.Public
	if a, _ = s.UpdateApplication(ctx, a.ID, in); a.DeployKey.Public != first {
		t.Fatal("an update must keep the deploy key")
	}
	if a, _ = s.RegenerateDeployKey(ctx, a.ID); a.DeployKey.Public == first {
		t.Fatal("regenerate must change the key")
	}

	in.GitURL = "https://example.com/r.git"
	if a, _ = s.UpdateApplication(ctx, a.ID, in); a.DeployKey != (domain.DeployKey{}) {
		t.Fatalf("back to https must drop the key: %+v", a.DeployKey)
	}
}

func TestDomainsChanged(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, fakeKey, "example.com", "")
	var events [][]string
	s.OnApplicationDomainsChanged(func(_ context.Context, id uint64, domains []string) {
		events = append(events, domains)
	})
	in := domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80}
	a, err := s.CreateApplication(ctx, 1, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateApplication(ctx, a.ID, in); err != nil || len(events) != 0 {
		t.Fatalf("unchanged domains: %v, events %v", err, events)
	}
	in.Domains = []string{"web.example.com", "www.example.com"}
	if _, err := s.UpdateApplication(ctx, a.ID, in); err != nil || len(events) != 1 || len(events[0]) != 2 {
		t.Fatalf("changed domains: %v, events %v", err, events)
	}
	in.Domains = []string{"taken.example.com"}
	var fe *domain.FieldError
	if _, err := s.UpdateApplication(ctx, a.ID, in); !errors.As(err, &fe) || fe.Field != "domains" || len(events) != 1 {
		t.Fatalf("taken domain: %v, events %v", err, events)
	}
}

// projectStore holds one Project and records whether it was deleted.
type projectStore struct {
	Store
	deleted bool
}

func (p *projectStore) Project(_ context.Context, id uint64) (domain.Project, bool, error) {
	return domain.Project{ID: 1, Name: "p"}, id == 1, nil
}
func (p *projectStore) DeleteProject(context.Context, uint64) error {
	p.deleted = true
	return nil
}

func TestDeleteProjectAsksInUseChecks(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	for _, tc := range []struct {
		name   string
		used   bool
		err    error
		want   error
		delete bool
	}{
		{name: "in use", used: true, want: ErrProjectNotEmpty},
		{name: "free", delete: true},
		{name: "check fails", err: boom, want: boom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &projectStore{}
			s := NewService(store, fakeKey, "localhost", "")
			var asked uint64
			s.OnProjectDeleting(func(_ context.Context, projectID uint64) (bool, error) {
				asked = projectID
				return tc.used, tc.err
			})
			err := s.DeleteProject(ctx, 1)
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Fatalf("DeleteProject: want %v, got %v", tc.want, err)
			}
			if asked != 1 || store.deleted != tc.delete {
				t.Fatalf("asked %d, deleted %v", asked, store.deleted)
			}
		})
	}
	if err := NewService(&projectStore{}, fakeKey, "localhost", "").DeleteProject(ctx, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project: %v", err)
	}
}
