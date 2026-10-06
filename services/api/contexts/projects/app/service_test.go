package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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

func (f *fakeStore) DeleteApplication(context.Context, uint64) error {
	f.app = domain.Application{}
	return nil
}

func TestDeleteApplicationPublishesWhatToRemove(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, fakeKey, "example.com", "")
	var events []ApplicationDeleted
	s.OnApplicationDeleted(func(_ context.Context, e ApplicationDeleted) { events = append(events, e) })
	a, err := s.CreateApplication(ctx, 1, domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteApplication(ctx, a.ID, false, true); err != nil {
		t.Fatal(err)
	}
	want := ApplicationDeleted{ApplicationID: a.ID, DeleteVolumes: false, DeleteImages: true}
	if len(events) != 1 || events[0] != want {
		t.Fatalf("events %+v, want %+v", events, want)
	}
	if err := s.DeleteApplication(ctx, a.ID, true, true); !errors.Is(err, ErrNotFound) || len(events) != 1 {
		t.Fatalf("deleting again: %v, events %+v", err, events)
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

func TestDomainChecks(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, fakeKey, "example.com", "bakery.example.com")
	boom := errors.New("boom")
	var checkErr error
	s.OnDomainCheck(func(_ context.Context, d string) (bool, error) {
		return d == "svc.example.com", checkErr
	})
	in := domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80, Domains: []string{"web.example.com", "svc.example.com"}}
	var fe *domain.FieldError
	if _, err := s.CreateApplication(ctx, 1, in); !errors.As(err, &fe) || !strings.Contains(fe.Message, "svc.example.com is already used by another application or service") {
		t.Fatalf("create with a Service's domain: %v", err)
	}
	in.Domains = []string{"web.example.com"}
	a, err := s.CreateApplication(ctx, 1, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Domains = []string{"svc.example.com"}
	if _, err := s.UpdateApplication(ctx, a.ID, in); !errors.As(err, &fe) {
		t.Fatalf("update to a Service's domain: %v", err)
	}
	checkErr = boom
	if _, err := s.UpdateApplication(ctx, a.ID, in); !errors.Is(err, boom) {
		t.Fatalf("a failing check: %v", err)
	}

	for d, want := range map[string]bool{"taken.example.com": true, "Bakery.example.com": true, "free.example.com": false} {
		if got, err := s.DomainInUse(ctx, d); err != nil || got != want {
			t.Errorf("DomainInUse(%s) = %v, %v", d, got, err)
		}
	}
}

func TestTargetServer(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	s := NewService(store, fakeKey, "example.com", "")
	s.LocalServer = func(context.Context) (uint64, error) { return 1, nil }
	s.ServerExists = func(_ context.Context, id uint64) (bool, error) { return id == 1 || id == 7, nil }
	in := domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80}

	if a, err := s.CreateApplication(ctx, 1, in); err != nil || a.ServerID != 0 {
		t.Fatalf("none chosen: %v, server %d", err, a.ServerID)
	}
	in.ServerID = 1
	if a, err := s.CreateApplication(ctx, 1, in); err != nil || a.ServerID != 0 {
		t.Fatalf("the Local server by id must be stored as 0: %v, server %d", err, a.ServerID)
	}
	in.ServerID = 99
	var fe *domain.FieldError
	if _, err := s.CreateApplication(ctx, 1, in); !errors.As(err, &fe) || fe.Field != "server_id" {
		t.Fatalf("unknown server: %v", err)
	}
	in.ServerID = 7
	a, err := s.CreateApplication(ctx, 1, in)
	if err != nil || a.ServerID != 7 {
		t.Fatalf("remote server: %v, server %d", err, a.ServerID)
	}

	// An update never moves the Application, whatever it sends.
	in.ServerID = 1
	in.Port = 8080
	if a, err = s.UpdateApplication(ctx, a.ID, in); err != nil || a.ServerID != 7 || a.Port != 8080 {
		t.Fatalf("update: %v, server %d", err, a.ServerID)
	}
}

// envStore keeps Projects' Environments in memory for the Environment use
// cases; anything else panics through the nil embedded Store.
type envStore struct {
	Store
	envs    map[uint64]domain.Environment
	hasApps map[uint64]bool
	next    uint64
}

func newEnvStore() *envStore {
	return &envStore{envs: map[uint64]domain.Environment{1: {ID: 1, ProjectID: 1, Name: domain.DefaultEnvironment}}, hasApps: map[uint64]bool{}, next: 1}
}

func (f *envStore) Project(_ context.Context, id uint64) (domain.Project, bool, error) {
	p := domain.Project{ID: id, Name: "shop"}
	for _, e := range f.envs {
		if e.ProjectID == id {
			p.Environments = append(p.Environments, e)
		}
	}
	return p, id == 1, nil
}
func (f *envStore) Environment(_ context.Context, id uint64) (domain.Environment, bool, error) {
	e, ok := f.envs[id]
	return e, ok, nil
}
func (f *envStore) EnvironmentNameTaken(_ context.Context, projectID uint64, name string, exceptID uint64) (bool, error) {
	for _, e := range f.envs {
		if e.ProjectID == projectID && e.ID != exceptID && strings.EqualFold(e.Name, name) {
			return true, nil
		}
	}
	return false, nil
}
func (f *envStore) CreateEnvironment(_ context.Context, e domain.Environment) (domain.Environment, error) {
	f.next++
	e.ID = f.next
	f.envs[e.ID] = e
	return e, nil
}
func (f *envStore) UpdateEnvironment(_ context.Context, e domain.Environment) error {
	f.envs[e.ID] = e
	return nil
}
func (f *envStore) DeleteEnvironment(_ context.Context, id uint64) error {
	if f.hasApps[id] {
		return ErrEnvironmentNotEmpty
	}
	delete(f.envs, id)
	return nil
}

func TestEnvironments(t *testing.T) {
	ctx := context.Background()
	store := newEnvStore()
	s := NewService(store, fakeKey, "example.com", "")
	var fe *domain.FieldError

	if _, err := s.CreateEnvironment(ctx, 2, "staging", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project: %v", err)
	}
	staging, err := s.CreateEnvironment(ctx, 1, " staging ", "pre-production")
	if err != nil || staging.Name != "staging" || staging.ProjectID != 1 || staging.Description != "pre-production" {
		t.Fatalf("create: %+v, %v", staging, err)
	}
	if _, err := s.CreateEnvironment(ctx, 1, "Staging", ""); !errors.As(err, &fe) || fe.Field != "name" {
		t.Fatalf("a second Staging: want a name error, got %v", err)
	}
	if _, err := s.UpdateEnvironment(ctx, staging.ID, "PRODUCTION", ""); !errors.As(err, &fe) || fe.Field != "name" {
		t.Fatalf("rename onto production: want a name error, got %v", err)
	}
	renamed, err := s.UpdateEnvironment(ctx, staging.ID, "Staging", "")
	if err != nil || renamed.Name != "Staging" || renamed.ProjectID != 1 {
		t.Fatalf("rename to another case of its own name: %+v, %v", renamed, err)
	}

	used := true
	s.OnEnvironmentDeleting(func(_ context.Context, id uint64) (bool, error) { return used && id == staging.ID, nil })
	if err := s.DeleteEnvironment(ctx, staging.ID); !errors.Is(err, ErrEnvironmentNotEmpty) {
		t.Fatalf("delete with a Database in it: %v", err)
	}
	used = false
	store.hasApps[staging.ID] = true
	if err := s.DeleteEnvironment(ctx, staging.ID); !errors.Is(err, ErrEnvironmentNotEmpty) {
		t.Fatalf("delete with an Application in it: %v", err)
	}
	store.hasApps[staging.ID] = false
	if err := s.DeleteEnvironment(ctx, staging.ID); err != nil {
		t.Fatalf("delete an empty one: %v", err)
	}
	if err := s.DeleteEnvironment(ctx, staging.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if err := s.DeleteEnvironment(ctx, 1); err != nil {
		t.Fatalf("the last Environment may go: %v", err)
	}
}

func TestEnvironmentInProject(t *testing.T) {
	ctx := context.Background()
	s := NewService(newEnvStore(), fakeKey, "example.com", "")
	p, e, err := s.EnvironmentInProject(ctx, 1)
	if err != nil || p.Name != "shop" || e.Name != domain.DefaultEnvironment {
		t.Fatalf("got %+v, %+v, %v", p, e, err)
	}
	if _, _, err := s.EnvironmentInProject(ctx, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestCreateApplicationWithoutAName(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, fakeKey, "example.com", "")
	generated := regexp.MustCompile(`^whoami:master-[a-z2-7]{8}$`)
	a, err := s.CreateApplication(ctx, 1, domain.ApplicationInput{GitURL: "https://github.com/traefik/whoami", GitBranch: "master", Port: 80})
	if err != nil || !generated.MatchString(a.Name) {
		t.Fatalf("git repository: %v, name %q", err, a.Name)
	}
	if a.Slug != domain.Slugify(a.Name) || a.Domains[0] != a.Slug+".example.com" {
		t.Errorf("slug %q, domains %q", a.Slug, a.Domains)
	}
	a, err = s.CreateApplication(ctx, 1, domain.ApplicationInput{Name: "  ", BuildPack: domain.DockerImage, DockerImage: "docker.io/traefik/whoami:v1.10", Port: 80})
	if err != nil || !regexp.MustCompile(`^docker-image-[a-z2-7]{8}$`).MatchString(a.Name) {
		t.Fatalf("docker image: %v, name %q", err, a.Name)
	}
}

func TestApplicationDescription(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, fakeKey, "example.com", "")
	described := "  The shop front  "
	in := domain.ApplicationInput{Name: "web", Description: &described, GitURL: "https://example.com/r.git", Port: 80}
	a, err := s.CreateApplication(ctx, 1, in)
	if err != nil || a.Description != "The shop front" {
		t.Fatalf("create: %v, %q", err, a.Description)
	}

	in.Description = nil
	if a, err = s.UpdateApplication(ctx, a.ID, in); err != nil || a.Description != "The shop front" {
		t.Fatalf("an update without a description must keep it: %v, %q", err, a.Description)
	}

	long := strings.Repeat("é", 256)
	in.Description = &long
	var fe *domain.FieldError
	if _, err = s.UpdateApplication(ctx, a.ID, in); !errors.As(err, &fe) || fe.Field != "description" {
		t.Fatalf("256 characters: want a description FieldError, got %v", err)
	}

	empty := ""
	in.Description = &empty
	if a, err = s.UpdateApplication(ctx, a.ID, in); err != nil || a.Description != "" {
		t.Fatalf("clearing: %v, %q", err, a.Description)
	}
}

func TestInGuildScopesAContext(t *testing.T) {
	if _, ok := GuildOf(context.Background()); ok {
		t.Fatal("a background context is scoped to a Guild")
	}
	if g, ok := GuildOf(InGuild(context.Background(), 7)); !ok || g != 7 {
		t.Fatalf("GuildOf(InGuild(7)) = %d, %v", g, ok)
	}
}

// guildStore keeps one Environment, in Guild 1, and honours the scope as the
// real store does.
type guildStore struct{ fakeStore }

func (g *guildStore) Environment(ctx context.Context, id uint64) (domain.Environment, bool, error) {
	if guild, ok := GuildOf(ctx); ok && guild != 1 {
		return domain.Environment{}, false, nil
	}
	return domain.Environment{ID: 1, ProjectID: 1, GuildID: 1, Name: domain.DefaultEnvironment}, true, nil
}

func TestAnApplicationBelongsToItsEnvironmentsGuild(t *testing.T) {
	s := NewService(&guildStore{}, fakeKey, "example.com", "")
	in := domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80}

	if _, err := s.CreateApplication(InGuild(context.Background(), 2), 1, in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create in another Guild's Environment: want ErrNotFound, got %v", err)
	}
	a, err := s.CreateApplication(InGuild(context.Background(), 1), 1, in)
	if err != nil {
		t.Fatal(err)
	}
	if a.GuildID != 1 {
		t.Fatalf("GuildID = %d, want 1", a.GuildID)
	}
}
