package app

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

type fakeStore struct {
	mu       sync.Mutex
	services map[uint64]domain.Service
	next     uint64
}

func newFakeStore() *fakeStore { return &fakeStore{services: map[uint64]domain.Service{}} }

// clone copies the slices, as a real store hands out fresh values.
func clone(s domain.Service) domain.Service {
	s.Components = slices.Clone(s.Components)
	for i := range s.Components {
		s.Components[i].Domains = slices.Clone(s.Components[i].Domains)
	}
	s.Variables = slices.Clone(s.Variables)
	return s
}

func (f *fakeStore) Create(_ context.Context, s domain.Service) (domain.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	s.ID = f.next
	f.services[s.ID] = clone(s)
	return s, nil
}
func (f *fakeStore) Get(_ context.Context, id uint64) (domain.Service, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.services[id]
	return clone(s), ok, nil
}
func (f *fakeStore) ForProject(_ context.Context, projectID uint64) ([]domain.Service, error) {
	return nil, nil
}
func (f *fakeStore) Wanted(context.Context) ([]domain.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Service
	for _, s := range f.services {
		if s.DesiredState == domain.Running {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeStore) Save(_ context.Context, s domain.Service) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[s.ID] = clone(s)
	return nil
}
func (f *fakeStore) SetLastError(_ context.Context, id uint64, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.services[id]
	s.LastError = msg
	f.services[id] = s
	return nil
}
func (f *fakeStore) SetDesiredState(_ context.Context, id uint64, st domain.DesiredState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.services[id]
	s.DesiredState = st
	f.services[id] = s
	return nil
}
func (f *fakeStore) Delete(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.services, id)
	return nil
}
func (f *fakeStore) SlugTaken(_ context.Context, slug string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.services {
		if s.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeStore) CountForProject(_ context.Context, projectID uint64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, s := range f.services {
		if s.ProjectID == projectID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) CountForEnvironment(_ context.Context, environmentID uint64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, s := range f.services {
		if s.EnvironmentID == environmentID {
			n++
		}
	}
	return n, nil
}
func (f *fakeStore) DomainTaken(_ context.Context, d string, except uint64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.services {
		if s.ID != except && slices.Contains(s.Domains(), d) {
			return true, nil
		}
	}
	return false, nil
}

// fakeRuntime blocks Up until release is closed, when set.
type fakeRuntime struct {
	mu      sync.Mutex
	ups     int
	pulls   int
	downs   int
	removed []uint64
	release chan struct{}
	upErr   error
}

func (f *fakeRuntime) Up(ctx context.Context, s domain.Service, _ domain.Compose, pull bool) error {
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ups++
	if pull {
		f.pulls++
	}
	return f.upErr
}
func (f *fakeRuntime) Down(context.Context, domain.Service) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downs++
	return nil
}
func (f *fakeRuntime) Remove(_ context.Context, s domain.Service) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, s.ID)
	return nil
}
func (f *fakeRuntime) Statuses(_ context.Context, s domain.Service) (map[string]domain.Status, map[string]string, error) {
	out := map[string]domain.Status{}
	for _, c := range s.Components {
		out[c.Name] = domain.StatusMissing
	}
	return out, nil, nil
}
func (f *fakeRuntime) Logs(context.Context, domain.Service, string, bool, int, func(string, string)) (bool, error) {
	return false, nil
}

type fakeRoutes struct {
	mu   sync.Mutex
	sets map[uint64][]Route
}

func (f *fakeRoutes) Set(_ context.Context, id uint64, rs []Route) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets[id] = rs
	return nil
}
func (f *fakeRoutes) Drop(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sets, id)
	return nil
}

const webCompose = "services:\n  web:\n    image: docker.io/traefik/whoami:v1.10\n    environment:\n      - SERVICE_FQDN_WEB_80\n      - SECRET=${SERVICE_PASSWORD_WEB}\n"

func newTestService(rt *fakeRuntime) (*Service, *fakeStore, *fakeRoutes) {
	store := newFakeStore()
	routes := &fakeRoutes{sets: map[uint64][]Route{}}
	envs := func(_ context.Context, id uint64) (Environment, error) {
		if id != 1 {
			return Environment{}, ErrNotFound
		}
		return Environment{ID: 1, ProjectID: 7}, nil
	}
	appDomain := func(_ context.Context, d string) (bool, error) {
		return d == "app.example.com" || d == "app.localhost", nil
	}
	gen := func(k domain.MagicKind) string { return "generated-" + string(k) }
	return NewService(store, rt, routes, envs, appDomain, "localhost", gen), store, routes
}

func TestCreateBringsUpAndRoutes(t *testing.T) {
	ctx := context.Background()
	rt := &fakeRuntime{}
	s, store, routes := newTestService(rt)
	v, err := s.Create(ctx, 1, Input{Name: "Who Am I", Compose: webCompose})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if v.Slug != "who-am-i" || v.ProjectID != 7 || rt.ups != 1 || rt.pulls != 1 {
		t.Fatalf("view %+v, ups %d pulls %d", v.Service, rt.ups, rt.pulls)
	}
	want := []Route{{Component: "web", Domains: []string{"who-am-i.localhost"}, Container: "bakery-svc-1-web", Port: 80}}
	if got := routes.sets[v.ID]; len(got) != 1 || got[0].Container != want[0].Container || got[0].Domains[0] != want[0].Domains[0] || got[0].Port != 80 {
		t.Errorf("routes %+v", got)
	}
	if sv, _, _ := store.Get(ctx, v.ID); sv.Variables[0].Value != "generated-password" {
		t.Errorf("variables %+v", sv.Variables)
	}

	// A second one with the same name gets its own slug and Domain.
	v2, err := s.Create(ctx, 1, Input{Name: "Who Am I", Compose: webCompose})
	if err != nil || v2.Slug != "who-am-i-2" {
		t.Fatalf("second: %v %+v", err, v2.Service)
	}
	s.Wait()

	// An Application with the default Domain moves the slug on too.
	v3, err := s.Create(ctx, 1, Input{Name: "app", Compose: webCompose})
	if err != nil || v3.Slug != "app-2" || v3.Components[0].Domains[0] != "app-2.localhost" {
		t.Fatalf("third: %v %+v", err, v3.Service)
	}
	s.Wait()

	if used, _ := s.InUse(ctx, 7); !used {
		t.Error("InUse is false with Services in the Project")
	}
	if used, _ := s.DomainInUse(ctx, "who-am-i.localhost"); !used {
		t.Error("DomainInUse misses a Service's Domain")
	}
}

func TestRefusals(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService(&fakeRuntime{})
	var ce *domain.ComposeFileError
	if _, err := s.Create(ctx, 1, Input{Name: "x", Compose: "services:\n  app:\n    build: .\n"}); !errors.As(err, &ce) {
		t.Errorf("build: %v", err)
	}
	if _, err := s.Create(ctx, 2, Input{Name: "x", Compose: webCompose}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown environment: %v", err)
	}
	v, err := s.Create(ctx, 1, Input{Name: "x", Compose: webCompose})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	var fe *domain.FieldError
	_, err = s.Update(ctx, v.ID, Change{Domains: map[string][]string{"web": {"app.example.com"}}})
	if !errors.As(err, &fe) || fe.Field != "domains" || !strings.Contains(fe.Message, "another application or service") {
		t.Errorf("an Application's domain: %v", err)
	}
	other, _ := s.Create(ctx, 1, Input{Name: "y", Compose: webCompose})
	s.Wait()
	if _, err := s.Update(ctx, other.ID, Change{Domains: map[string][]string{"web": {"x.localhost"}}}); !errors.As(err, &fe) {
		t.Errorf("another Service's domain: %v", err)
	}
	if _, err := s.Get(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown service: %v", err)
	}
}

func TestBusyAndStop(t *testing.T) {
	ctx := context.Background()
	rt := &fakeRuntime{release: make(chan struct{})}
	s, store, _ := newTestService(rt)
	v, err := s.Create(ctx, 1, Input{Name: "x", Compose: webCompose})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, v.ID); !got.Busy || got.Status != domain.ServiceDeploying {
		t.Errorf("while up: busy %v status %s", got.Busy, got.Status)
	}
	if _, err := s.Redeploy(ctx, v.ID); !errors.Is(err, ErrBusy) {
		t.Errorf("redeploy while busy: %v", err)
	}
	// Stop cancels the running Up and removes the Containers.
	if _, err := s.Stop(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	sv, _, _ := store.Get(ctx, v.ID)
	if sv.DesiredState != domain.Stopped || rt.downs != 1 || sv.LastError != "" {
		t.Errorf("after stop: %+v downs %d", sv, rt.downs)
	}

	close(rt.release)
	rt.upErr = errors.New("pull refused")
	if _, err := s.Start(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	sv, _, _ = store.Get(ctx, v.ID)
	if sv.DesiredState != domain.Running || !strings.Contains(sv.LastError, "start failed: pull refused") {
		t.Errorf("failed start: %+v", sv)
	}

	rt.upErr = nil
	if err := s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if sv, _, _ = store.Get(ctx, v.ID); sv.LastError != "" {
		t.Errorf("recover did not clear the error: %q", sv.LastError)
	}
	if err := s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, v.ID); found || !slices.Contains(rt.removed, v.ID) {
		t.Errorf("delete left it: removed %v", rt.removed)
	}
}

func TestUpdateKeepsGeneratedValuesAndMovesRoutes(t *testing.T) {
	ctx := context.Background()
	s, store, routes := newTestService(&fakeRuntime{})
	v, _ := s.Create(ctx, 1, Input{Name: "x", Compose: webCompose})
	s.Wait()
	name := "Renamed"
	if _, err := s.Update(ctx, v.ID, Change{Name: &name, Domains: map[string][]string{"web": {"who.example.com"}}}); err != nil {
		t.Fatal(err)
	}
	if got := routes.sets[v.ID]; len(got) != 1 || got[0].Domains[0] != "who.example.com" {
		t.Errorf("routes did not move: %+v", got)
	}
	changed := webCompose + "      - EXTRA=${EXTRA:-1}\n"
	if _, err := s.Update(ctx, v.ID, Change{Compose: &changed, Variables: map[string]string{"EXTRA": "2"}}); err != nil {
		t.Fatal(err)
	}
	sv, _, _ := store.Get(ctx, v.ID)
	values := sv.Values()
	if sv.Name != "Renamed" || values["SERVICE_PASSWORD_WEB"] != "generated-password" || values["EXTRA"] != "2" || sv.Domains()[0] != "who.example.com" {
		t.Errorf("after update: %+v %v", sv, values)
	}
	if _, err := s.Update(ctx, v.ID, Change{Variables: map[string]string{"SERVICE_PASSWORD_WEB": "x"}}); err == nil {
		t.Error("set a magic variable")
	}
}

func TestCreateFromTemplate(t *testing.T) {
	ctx := context.Background()
	s, store, _ := newTestService(&fakeRuntime{})
	s.SetTemplates([]domain.Template{{Key: "who", Name: "Who", Compose: webCompose}})
	v, err := s.CreateFromTemplate(ctx, 1, "who", "")
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if sv, _, _ := store.Get(ctx, v.ID); sv.Name != "Who" || sv.TemplateKey != "who" || sv.ComposeFile != webCompose {
		t.Errorf("created %+v", sv)
	}
	var fe *domain.FieldError
	if _, err := s.CreateFromTemplate(ctx, 1, "nope", ""); !errors.As(err, &fe) || fe.Field != "template" {
		t.Errorf("unknown template: %v", err)
	}
}

func TestCreateWithoutAName(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService(&fakeRuntime{})
	v, err := s.Create(ctx, 1, Input{Compose: webCompose})
	s.Wait()
	if err != nil || !regexp.MustCompile(`^docker-compose-[a-z2-7]{8}$`).MatchString(v.Name) || v.Slug != v.Name {
		t.Fatalf("pasted compose: %v, name %q, slug %q", err, v.Name, v.Slug)
	}
}
