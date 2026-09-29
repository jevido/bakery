package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type memStore struct {
	mu    sync.Mutex
	items []domain.Deployment
	saves []domain.Status
}

func (m *memStore) Queue(_ context.Context, appID uint64) (domain.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.items {
		if d.ApplicationID == appID && d.Status.Active() {
			return domain.Deployment{}, domain.ErrActiveDeployment
		}
	}
	d := domain.Deployment{ID: uint64(len(m.items) + 1), ApplicationID: appID, Status: domain.Queued}
	m.items = append(m.items, d)
	return d, nil
}

func (m *memStore) ClaimNext(context.Context) (domain.Deployment, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, d := range m.items {
		if d.Status == domain.Queued {
			m.items[i].Status = domain.Cloning
			return m.items[i], true, nil
		}
	}
	return domain.Deployment{}, false, nil
}

func (m *memStore) Save(_ context.Context, d domain.Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[d.ID-1] = d
	m.saves = append(m.saves, d.Status)
	return nil
}

func (m *memStore) FailInterrupted(context.Context, string) (int, error) { return 0, nil }
func (m *memStore) ByID(_ context.Context, id uint64) (domain.Deployment, bool, error) {
	return m.items[id-1], true, nil
}
func (m *memStore) ByApplication(context.Context, uint64, int) ([]domain.Deployment, error) {
	return m.items, nil
}
func (m *memStore) Active(context.Context, uint64) (domain.Deployment, bool, error) {
	return domain.Deployment{}, false, nil
}
func (m *memStore) DeleteForApplication(context.Context, uint64) error { return nil }

type memLogs struct {
	mu    sync.Mutex
	lines []string
}

func (l *memLogs) Writer(uint64) LogWriter { return l }
func (l *memLogs) Line(stream, line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, stream+": "+line)
}
func (l *memLogs) Close() error { return nil }
func (l *memLogs) After(context.Context, uint64, uint64, int) ([]domain.LogLine, error) {
	return nil, nil
}
func (l *memLogs) text() string { return strings.Join(l.lines, "\n") }

type fakeSource struct{ noDockerfile, fail bool }

func (f fakeSource) Clone(_ context.Context, _, _, dir string, out func(string, string)) (string, error) {
	if f.fail {
		return "", errors.New("Remote branch nope not found")
	}
	out(domain.StreamErr, "Cloning into...")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if !f.noDockerfile {
		os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch"), 0o600)
	}
	return "0123456789abcdef0123456789abcdef01234567", nil
}

type fakeRuntime struct {
	buildErr, startErr error
	running            map[string]bool
	removed            []string
}

func (r *fakeRuntime) Build(_ context.Context, _, _, _ string, _ map[string]string, out func(string)) error {
	out("STEP 1/1: FROM scratch")
	return r.buildErr
}
func (r *fakeRuntime) Start(_ context.Context, s ContainerSpec) error {
	if r.startErr != nil {
		return r.startErr
	}
	r.running[s.Name] = true
	return nil
}
func (r *fakeRuntime) Remove(_ context.Context, name string) error {
	delete(r.running, name)
	r.removed = append(r.removed, name)
	return nil
}
func (r *fakeRuntime) RemoveOthers(_ context.Context, _ uint64, keep string) ([]string, error) {
	var out []string
	for name := range r.running {
		if name != keep {
			out = append(out, name)
			delete(r.running, name)
		}
	}
	return out, nil
}
func (r *fakeRuntime) RemoveAll(context.Context, uint64) error { return nil }

type setup struct {
	store   *memStore
	logs    *memLogs
	runtime *fakeRuntime
	routes  map[string]string
	service *Service
	worker  *Worker
}

func newSetup(t *testing.T, src fakeSource) *setup {
	s := &setup{store: &memStore{}, logs: &memLogs{}, runtime: &fakeRuntime{running: map[string]bool{}}, routes: map[string]string{}}
	apps := func(_ context.Context, id uint64) (Application, error) {
		return Application{ID: id, Slug: "whoami", GitURL: "https://example.com/r", GitBranch: "main", DockerfilePath: "Dockerfile", Port: 80, Domain: "whoami.localhost", Env: map[string]string{"HELLO": "world"}}, nil
	}
	s.service = NewService(s.store, s.logs, apps)
	router := func(_ context.Context, _ uint64, domain, container string, _ int) error {
		s.routes[domain] = container
		return nil
	}
	s.worker = NewWorker(s.service, src, s.runtime, router, t.TempDir())
	return s
}

func TestDeploySucceedsAndReplacesOldContainer(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.runtime.running["bakery-app-1-0"] = true // a previous deployment

	d, err := s.service.Deploy(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.service.Deploy(ctx, 1); !errors.Is(err, domain.ErrActiveDeployment) {
		t.Fatalf("second deploy while active: %v", err)
	}
	if !s.worker.RunOnce(ctx) {
		t.Fatal("nothing claimed")
	}
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished || got.CommitSHA == "" || got.Container != "bakery-app-1-1" || got.FinishedAt == nil {
		t.Fatalf("deployment: %+v\n%s", got, s.logs.text())
	}
	if s.routes["whoami.localhost"] != "bakery-app-1-1" {
		t.Fatalf("route: %v", s.routes)
	}
	if s.runtime.running["bakery-app-1-0"] || !s.runtime.running["bakery-app-1-1"] {
		t.Fatalf("containers: %v", s.runtime.running)
	}
	for _, want := range []string{"info: Cloning https://example.com/r", "err: Cloning into", "out: STEP 1/1", "Removed previous container bakery-app-1-0", "Deployment finished"} {
		if !strings.Contains(s.logs.text(), want) {
			t.Errorf("log lacks %q:\n%s", want, s.logs.text())
		}
	}
	want := []domain.Status{domain.Building, domain.Starting, domain.Finished}
	if strings.Join(statuses(s.store.saves), ",") != strings.Join(statuses(want), ",") {
		t.Errorf("saved statuses %v, want %v", s.store.saves, want)
	}
	if _, err := s.service.Deploy(ctx, 1); err != nil {
		t.Fatalf("deploy after finish: %v", err)
	}
}

func statuses(ss []domain.Status) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = string(s)
	}
	return out
}

func TestDeployFailures(t *testing.T) {
	cases := map[string]struct {
		src     fakeSource
		runtime func(*fakeRuntime)
		want    string
	}{
		"clone":         {src: fakeSource{fail: true}, want: "clone failed: Remote branch nope not found"},
		"no dockerfile": {src: fakeSource{noDockerfile: true}, want: "no Dockerfile in the repository at 0123456789ab"},
		"build":         {runtime: func(r *fakeRuntime) { r.buildErr = errors.New("exit status 1") }, want: "build failed: exit status 1"},
		"start":         {runtime: func(r *fakeRuntime) { r.startErr = errors.New("exited with code 1") }, want: "container did not start: exited with code 1"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := newSetup(t, c.src)
			s.runtime.running["bakery-app-1-0"] = true
			if c.runtime != nil {
				c.runtime(s.runtime)
			}
			d, _ := s.service.Deploy(ctx, 1)
			s.worker.RunOnce(ctx)
			got, _ := s.service.Deployment(ctx, d.ID)
			if got.Status != domain.Failed || got.Error != c.want {
				t.Fatalf("got %s %q, want failed %q", got.Status, got.Error, c.want)
			}
			if !s.runtime.running["bakery-app-1-0"] {
				t.Fatal("a failed deployment must leave the running container alone")
			}
			if len(s.routes) != 0 {
				t.Fatalf("route switched on failure: %v", s.routes)
			}
		})
	}
}
