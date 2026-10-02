package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type memStore struct {
	mu    sync.Mutex
	items []domain.Deployment
	saves []domain.Status
}

func (m *memStore) Queue(_ context.Context, d domain.Deployment) (domain.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range m.items {
		if q.ApplicationID == d.ApplicationID && q.Preview == d.Preview && q.Status == domain.Queued {
			return domain.Deployment{}, domain.ErrAlreadyQueued
		}
	}
	d.ID = uint64(len(m.items) + 1)
	m.items = append(m.items, d)
	return d, nil
}

func (m *memStore) ClaimNext(context.Context) (domain.Deployment, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	running := map[uint64]bool{}
	for _, d := range m.items {
		if d.Status.Active() && d.Status != domain.Queued {
			running[d.ApplicationID] = true
		}
	}
	for i, d := range m.items {
		if d.Status == domain.Queued && !running[d.ApplicationID] {
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

func (m *memStore) CancelQueued(_ context.Context, id uint64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items[id-1].Status != domain.Queued {
		return false, nil
	}
	m.items[id-1].Status = domain.Cancelled
	return true, nil
}

func (m *memStore) FailInterrupted(context.Context, string) (int, error) { return 0, nil }
func (m *memStore) ByID(_ context.Context, id uint64) (domain.Deployment, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[id-1], true, nil
}
func (m *memStore) ByApplication(context.Context, uint64, int) ([]domain.Deployment, error) {
	return m.items, nil
}
func (m *memStore) ByPreview(_ context.Context, _ uint64, number int, _ int) ([]domain.Deployment, error) {
	var out []domain.Deployment
	for _, d := range m.items {
		if d.Preview == number {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memStore) Active(context.Context, uint64) (domain.Deployment, bool, error) {
	return domain.Deployment{}, false, nil
}
func (m *memStore) DeleteForApplication(context.Context, uint64) error  { return nil }
func (m *memStore) ServerIDs(context.Context, uint64) ([]uint64, error) { return nil, nil }
func (m *memStore) ApplicationIDs(context.Context) ([]uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[uint64]bool{}
	var ids []uint64
	for _, d := range m.items {
		if !seen[d.ApplicationID] {
			seen[d.ApplicationID] = true
			ids = append(ids, d.ApplicationID)
		}
	}
	return ids, nil
}

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

type fakeSource struct {
	noDockerfile, fail bool
	// files are written into the clone, by slash path.
	files map[string]string
}

func (f fakeSource) Clone(_ context.Context, req CloneRequest, out func(string, string)) (Commit, error) {
	dir := req.Dir
	if f.fail {
		return Commit{}, errors.New("Remote branch nope not found")
	}
	out(domain.StreamErr, "Cloning into...")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Commit{}, err
	}
	if !f.noDockerfile {
		os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch"), 0o600)
	}
	for name, body := range f.files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte(body), 0o600)
	}
	return Commit{SHA: "0123456789abcdef0123456789abcdef01234567", Subject: "Fix the login", Author: "Jane Doe"}, nil
}

func (r *fakeRuntime) ServerName() string { return "fake" }

type fakeRuntime struct {
	buildErr, startErr error
	// building, when set, is closed once Build started, and Build then
	// blocks until its ctx ends.
	building chan struct{}
	running  map[string]bool
	removed  []string
	// probes answers each Probe in turn; the last one repeats.
	probes []probe
	probed []string
	specs  []ContainerSpec
	built  int
	builds []BuildRequest
	// gone lists Images that no longer exist.
	gone    map[string]bool
	pulls   []PullRequest
	pullErr error
	// previewOf is the Preview number of each Container Start made.
	previewOf map[string]int
}

func (r *fakeRuntime) Pull(_ context.Context, req PullRequest, out func(string)) (string, error) {
	r.pulls = append(r.pulls, req)
	out("Copying blob 1234")
	if r.pullErr != nil {
		return "", r.pullErr
	}
	return "docker.io/traefik/whoami@sha256:abc", nil
}

func (r *fakeRuntime) ImageExists(_ context.Context, image string) (bool, error) {
	return !r.gone[image], nil
}

func (r *fakeRuntime) RemoveImage(_ context.Context, image string) (int64, error) {
	if r.gone == nil {
		r.gone = map[string]bool{}
	}
	r.gone[image] = true
	return 100, nil
}

type probe struct {
	ok     bool
	detail string
	err    error
}

func (r *fakeRuntime) Probe(_ context.Context, container, url string, _ time.Duration) (bool, string, error) {
	r.probed = append(r.probed, container+" "+url)
	p := r.probes[min(len(r.probed), len(r.probes))-1]
	return p.ok, p.detail, p.err
}

func (r *fakeRuntime) Build(ctx context.Context, req BuildRequest, out func(string)) error {
	r.built++
	r.builds = append(r.builds, req)
	out("STEP 1/1: FROM scratch")
	if r.building != nil {
		close(r.building)
		<-ctx.Done()
		return ctx.Err()
	}
	return r.buildErr
}
func (r *fakeRuntime) Start(_ context.Context, s ContainerSpec) error {
	r.specs = append(r.specs, s)
	if r.startErr != nil {
		return r.startErr
	}
	r.running[s.Name] = true
	if r.previewOf == nil {
		r.previewOf = map[string]int{}
	}
	r.previewOf[s.Name] = s.Preview
	return nil
}
func (r *fakeRuntime) Remove(_ context.Context, name string) error {
	delete(r.running, name)
	r.removed = append(r.removed, name)
	return nil
}
func (r *fakeRuntime) RemoveOthers(_ context.Context, _ uint64, preview int, keep string) ([]string, error) {
	var out []string
	for name := range r.running {
		if name != keep && r.previewOf[name] == preview {
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
	// servers are the reachable Servers' Runtimes, the Local one (0) is
	// runtime; routedOn is the Server each Domain was routed on.
	servers  map[uint64]*fakeRuntime
	routedOn map[string]uint64
	service  *Service
	worker   *Worker
	previews *memPreviews
	// app, when set, changes the Application every Deployment reads.
	app func(*Application)
}

func newSetup(t *testing.T, src Source, check ...HealthCheck) *setup {
	s := &setup{store: &memStore{}, logs: &memLogs{}, runtime: &fakeRuntime{running: map[string]bool{}}, routes: map[string]string{}, servers: map[uint64]*fakeRuntime{}, routedOn: map[string]uint64{}}
	s.servers[0] = s.runtime
	apps := func(_ context.Context, id uint64) (Application, error) {
		a := Application{ID: id, Slug: "whoami", GitURL: "https://example.com/r", GitBranch: "main", DockerfilePath: "Dockerfile", Port: 80, Domains: []string{"whoami.localhost"}, RuntimeEnv: map[string]string{"HELLO": "world"}, BuildEnv: map[string]string{"VITE_API": "https://api", "B": "1"}}
		if len(check) > 0 {
			a.HealthCheck = check[0]
		}
		if s.app != nil {
			s.app(&a)
		}
		return a, nil
	}
	s.previews = &memPreviews{}
	s.service = NewService(s.store, s.logs, apps, nil, s.previews)
	router := func(_ context.Context, serverID, _ uint64, domains []string, container string, _ int) error {
		for _, d := range domains {
			s.routes[d] = container
			s.routedOn[d] = serverID
		}
		return nil
	}
	runtimes := func(_ context.Context, serverID uint64) (Runtime, error) {
		rt, ok := s.servers[serverID]
		if !ok {
			return nil, errors.New("server web is not reachable: connection refused")
		}
		return rt, nil
	}
	s.worker = NewWorker(s.service, src, runtimes, router, t.TempDir())
	s.worker.PreviewRouter = func(_ context.Context, serverID, _ uint64, _ int, domains []string, container string, _ int) error {
		for _, d := range domains {
			s.routes[d] = container
			s.routedOn[d] = serverID
		}
		return nil
	}
	s.worker.sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
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
	if _, err := s.service.Deploy(ctx, 1); !errors.Is(err, domain.ErrAlreadyQueued) {
		t.Fatalf("second deploy while queued: %v", err)
	}
	if !s.worker.RunOnce(ctx) {
		t.Fatal("nothing claimed")
	}
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished || got.CommitSHA == "" || got.Container != "bakery-app-1-1" || got.FinishedAt == nil {
		t.Fatalf("deployment: %+v\n%s", got, s.logs.text())
	}
	if got.Branch != "main" || got.CommitMessage != "Fix the login" || got.CommitAuthor != "Jane Doe" || got.Trigger != domain.TriggerManual {
		t.Fatalf("source details: %+v", got)
	}
	if s.routes["whoami.localhost"] != "bakery-app-1-1" {
		t.Fatalf("route: %v", s.routes)
	}
	if s.runtime.running["bakery-app-1-0"] || !s.runtime.running["bakery-app-1-1"] {
		t.Fatalf("containers: %v", s.runtime.running)
	}
	if b := s.runtime.builds[0]; b.BuildArgs["VITE_API"] != "https://api" || b.Tag != "localhost/bakery/whoami:1" || b.Dockerfile != "Dockerfile" {
		t.Errorf("build request %+v", b)
	}
	if env := s.runtime.specs[0].Env; env["HELLO"] != "world" || env["VITE_API"] != "" {
		t.Errorf("container env %v", env)
	}
	for _, want := range []string{"info: Build args: B, VITE_API", "info: Cloning https://example.com/r", "err: Cloning into", "out: STEP 1/1", `Checked out 0123456789ab "Fix the login" by Jane Doe`, "Removed previous container bakery-app-1-0", "Deployment finished"} {
		if !strings.Contains(s.logs.text(), want) {
			t.Errorf("log lacks %q:\n%s", want, s.logs.text())
		}
	}
	want := []domain.Status{domain.Cloning, domain.Building, domain.Starting, domain.Finished}
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

func TestDeployQueuesBehindARunningDeployment(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	first, _ := s.service.Deploy(ctx, 1)
	if _, found, _ := s.store.ClaimNext(ctx); !found {
		t.Fatal("first not claimed")
	}
	second, err := s.service.Deploy(ctx, 1)
	if err != nil {
		t.Fatalf("deploy while running: %v", err)
	}
	if _, err := s.service.Deploy(ctx, 1); !errors.Is(err, domain.ErrAlreadyQueued) {
		t.Fatalf("third deploy: %v", err)
	}
	if _, found, _ := s.store.ClaimNext(ctx); found {
		t.Fatal("claimed a queued deployment while its application has a running one")
	}
	// The first finishes; now the second is claimed.
	d, _ := s.service.Deployment(ctx, first.ID)
	d.Status = domain.Finished
	s.store.Save(ctx, d)
	if !s.worker.RunOnce(ctx) {
		t.Fatal("second not claimed after the first finished")
	}
	if got, _ := s.service.Deployment(ctx, second.ID); got.Status != domain.Finished {
		t.Fatalf("second: %s %s", got.Status, got.Error)
	}
}

var check = HealthCheck{Enabled: true, Path: "/health", Interval: 1, Timeout: 1, Retries: 3, StartPeriod: 5}

func TestDeployWaitsUntilHealthy(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{}, check)
	s.runtime.running["bakery-app-1-0"] = true
	s.runtime.probes = []probe{{detail: "curl: (7) Failed to connect"}, {detail: "HTTP 503"}, {ok: true}}
	d, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished {
		t.Fatalf("got %s %q\n%s", got.Status, got.Error, s.logs.text())
	}
	if len(s.runtime.probed) != 3 || s.runtime.probed[0] != "bakery-app-1-1 http://127.0.0.1:80/health" {
		t.Fatalf("probed %v", s.runtime.probed)
	}
	if s.runtime.specs[0].Settle {
		t.Error("settle asked for although a health check follows")
	}
	if s.routes["whoami.localhost"] != "bakery-app-1-1" || s.runtime.running["bakery-app-1-0"] {
		t.Fatalf("route %v, containers %v", s.routes, s.runtime.running)
	}
	for _, want := range []string{"Waiting 5s before the first health check", "Waiting for /health (attempt 1/3)", "Not healthy yet: HTTP 503", "Healthy after 3 attempt(s)"} {
		if !strings.Contains(s.logs.text(), want) {
			t.Errorf("log lacks %q:\n%s", want, s.logs.text())
		}
	}
}

func TestDeployWithoutHealthCheckSettles(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	if !s.runtime.specs[0].Settle || len(s.runtime.probed) != 0 {
		t.Fatalf("spec %+v, probed %v", s.runtime.specs[0], s.runtime.probed)
	}
}

func TestDeployUnhealthyKeepsTheOldContainer(t *testing.T) {
	cases := map[string]struct {
		probes []probe
		want   string
		tries  int
	}{
		"never healthy": {[]probe{{detail: "curl: (22) The requested URL returned error: 500"}}, "health check failed after 3 attempts: curl: (22) The requested URL returned error: 500", 3},
		"no tool":       {[]probe{{err: errors.New("the health check needs sh and curl or wget in the image")}}, "health check failed: the health check needs sh and curl or wget in the image", 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := newSetup(t, fakeSource{}, check)
			s.runtime.running["bakery-app-1-0"] = true
			s.runtime.probes = c.probes
			d, _ := s.service.Deploy(ctx, 1)
			s.worker.RunOnce(ctx)
			got, _ := s.service.Deployment(ctx, d.ID)
			if got.Status != domain.Failed || got.Error != c.want {
				t.Fatalf("got %s %q, want %q", got.Status, got.Error, c.want)
			}
			if len(s.runtime.probed) != c.tries {
				t.Errorf("probed %d times, want %d", len(s.runtime.probed), c.tries)
			}
			if !s.runtime.running["bakery-app-1-0"] || s.runtime.running["bakery-app-1-1"] {
				t.Fatalf("containers: %v", s.runtime.running)
			}
			if len(s.routes) != 0 {
				t.Fatalf("route switched: %v", s.routes)
			}
		})
	}
}

func TestCancelQueued(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	d, _ := s.service.Deploy(ctx, 1)
	got, err := s.service.Cancel(ctx, d.ID)
	if err != nil || got.Status != domain.Cancelled {
		t.Fatalf("cancel queued: %v %s", err, got.Status)
	}
	if s.worker.RunOnce(ctx) {
		t.Fatal("claimed a cancelled deployment")
	}
	if _, err := s.service.Cancel(ctx, d.ID); !errors.Is(err, ErrNotCancellable) {
		t.Fatalf("second cancel: %v", err)
	}
}

func TestCancelRunning(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.runtime.running["bakery-app-1-0"] = true
	s.runtime.building = make(chan struct{})
	d, _ := s.service.Deploy(ctx, 1)
	done := make(chan struct{})
	go func() {
		s.worker.RunOnce(ctx)
		close(done)
	}()
	<-s.runtime.building
	if _, err := s.service.Cancel(ctx, d.ID); err != nil {
		t.Fatalf("cancel running: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop")
	}
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Cancelled || got.FinishedAt == nil {
		t.Fatalf("got %s %q", got.Status, got.Error)
	}
	if !strings.Contains(s.logs.text(), "Deployment cancelled.") {
		t.Errorf("log:\n%s", s.logs.text())
	}
	if !s.runtime.running["bakery-app-1-0"] || len(s.routes) != 0 {
		t.Fatalf("containers %v, routes %v", s.runtime.running, s.routes)
	}
	if _, err := s.service.Deploy(ctx, 1); err != nil {
		t.Fatalf("deploy after cancel: %v", err)
	}
}

type countingSource struct {
	fakeSource
	clones *int
}

func (c countingSource) Clone(ctx context.Context, req CloneRequest, out func(string, string)) (Commit, error) {
	*c.clones++
	return c.fakeSource.Clone(ctx, req, out)
}

func TestRollback(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	clones := 0
	s.worker.source = countingSource{clones: &clones}
	first, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	if clones != 2 || s.routes["whoami.localhost"] != "bakery-app-1-2" {
		t.Fatalf("setup: clones %d, routes %v", clones, s.routes)
	}

	d, err := s.service.Rollback(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Trigger != domain.TriggerRollback || d.Image != "localhost/bakery/whoami:1" || d.CommitMessage != "Fix the login" {
		t.Fatalf("queued rollback: %+v", d)
	}
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished || got.Image != "localhost/bakery/whoami:1" || got.Container != "bakery-app-1-3" {
		t.Fatalf("rollback: %+v\n%s", got, s.logs.text())
	}
	if clones != 2 || s.runtime.built != 2 {
		t.Fatalf("rollback cloned or built: clones %d, builds %d", clones, s.runtime.built)
	}
	if s.routes["whoami.localhost"] != "bakery-app-1-3" || s.runtime.running["bakery-app-1-2"] {
		t.Fatalf("routes %v, containers %v", s.routes, s.runtime.running)
	}
	if last := s.runtime.specs[len(s.runtime.specs)-1]; last.Image != "localhost/bakery/whoami:1" || last.Env["HELLO"] != "world" {
		t.Fatalf("started %+v", last)
	}
	if !strings.Contains(s.logs.text(), "Rolling back to deployment 1") {
		t.Errorf("log:\n%s", s.logs.text())
	}
}

func TestRollbackRefused(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	ok, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	s.runtime.buildErr = errors.New("boom")
	failed, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)

	if _, err := s.service.Rollback(ctx, failed.ID); !errors.Is(err, domain.ErrNotRollbackTarget) {
		t.Fatalf("rollback to failed: %v", err)
	}
	s.runtime.gone = map[string]bool{"localhost/bakery/whoami:1": true}
	if _, err := s.service.Rollback(ctx, ok.ID); !errors.Is(err, ErrImageGone) {
		t.Fatalf("rollback to a removed image: %v", err)
	}
}

func imageApp(a *Application) {
	a.BuildPack, a.ImageReference = BuildPackImage, "docker.io/traefik/whoami:v1.10"
	a.GitURL, a.GitBranch, a.DockerfilePath = "", "", ""
	a.RegistryUsername, a.RegistryPassword = "me", "s3cret"
}

func TestImageBuildPackPullsInsteadOfBuilding(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{fail: true}) // a clone would fail
	s.app = imageApp
	d, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished || got.SourceImage != "docker.io/traefik/whoami@sha256:abc" || got.Image != "localhost/bakery/whoami:1" {
		t.Fatalf("deployment: %+v\n%s", got, s.logs.text())
	}
	if s.runtime.built != 0 {
		t.Errorf("built %d times", s.runtime.built)
	}
	want := PullRequest{Reference: "docker.io/traefik/whoami:v1.10", Tag: "localhost/bakery/whoami:1", Username: "me", Password: "s3cret"}
	if len(s.runtime.pulls) != 1 || s.runtime.pulls[0] != want {
		t.Errorf("pulls %+v", s.runtime.pulls)
	}
	if s.runtime.specs[0].Image != "localhost/bakery/whoami:1" || s.routes["whoami.localhost"] != "bakery-app-1-1" {
		t.Errorf("start %+v, routes %v", s.runtime.specs, s.routes)
	}
	text := s.logs.text()
	for _, w := range []string{"Pulling docker.io/traefik/whoami:v1.10 with registry credentials for me", "out: Copying blob", "Pulled docker.io/traefik/whoami@sha256:abc"} {
		if !strings.Contains(text, w) {
			t.Errorf("log lacks %q:\n%s", w, text)
		}
	}
	if strings.Contains(text, "s3cret") || strings.Contains(text, "Cloning") {
		t.Errorf("log has the password or a clone:\n%s", text)
	}

	// A Rollback shows the pulled image too.
	rb, err := s.service.Rollback(ctx, d.ID)
	if err != nil || rb.SourceImage != got.SourceImage {
		t.Fatalf("rollback %+v, %v", rb, err)
	}
}

func TestImageBuildPackPullFailure(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.app = imageApp
	s.runtime.pullErr = errors.New("unauthorized: authentication required")
	d, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Failed || got.Error != "pull failed: unauthorized: authentication required" {
		t.Fatalf("deployment: %+v", got)
	}
	if len(s.runtime.specs) != 0 {
		t.Errorf("started %+v", s.runtime.specs)
	}
}

func TestStaticBuildPack(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{noDockerfile: true, files: map[string]string{"public/index.html": "<h1>hi</h1>"}})
	var generated string
	s.app = func(a *Application) { a.BuildPack, a.PublishDirectory, a.Port = BuildPackStatic, "public", 80 }
	d, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished {
		t.Fatalf("deployment: %+v\n%s", got, s.logs.text())
	}
	b := s.runtime.builds[0]
	if b.Dockerfile != ".bakery-static-1.Containerfile" || len(b.BuildArgs) != 0 {
		t.Errorf("build request %+v", b)
	}
	generated = StaticContainerfile("public", b.Dockerfile)
	if !strings.Contains(generated, `COPY ["public/","/srv/"]`) || strings.Contains(generated, "rm -f") {
		t.Errorf("containerfile:\n%s", generated)
	}
	if !strings.Contains(StaticContainerfile(".", "x.Containerfile"), "RUN rm -f /srv/x.Containerfile") {
		t.Error("publishing the whole repository must not serve the Containerfile")
	}
	for _, w := range []string{"Serving public as a static site", "Build variables are not used"} {
		if !strings.Contains(s.logs.text(), w) {
			t.Errorf("log lacks %q:\n%s", w, s.logs.text())
		}
	}

	// A missing directory fails before building.
	s.app = func(a *Application) { a.BuildPack, a.PublishDirectory = BuildPackStatic, "dist" }
	d, _ = s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ = s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Failed || !strings.HasPrefix(got.Error, "no directory dist in the repository at 0123456789ab") || s.runtime.built != 1 {
		t.Fatalf("missing dir: %+v, built %d", got, s.runtime.built)
	}
}

type fakePlanner struct{ env map[string]string }

func (p *fakePlanner) Plan(_ context.Context, dir string, env map[string]string, out func(string, string)) (string, map[string]string, error) {
	p.env = env
	out(domain.StreamOut, "║ setup │ nodejs_18 ║")
	return ".nixpacks/Dockerfile", map[string]string{"NODE_ENV": "production", "VITE_API": env["VITE_API"]}, nil
}

func TestNixpacksBuildPack(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{noDockerfile: true})
	s.app = func(a *Application) { a.BuildPack = BuildPackNixpacks }
	d, _ := s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Failed || got.Error != "the nixpacks build pack is not available here" {
		t.Fatalf("without a planner: %+v", got)
	}

	p := &fakePlanner{}
	s.worker.Planner = p
	d, _ = s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	got, _ = s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished {
		t.Fatalf("deployment: %+v\n%s", got, s.logs.text())
	}
	b := s.runtime.builds[0]
	if b.Dockerfile != ".nixpacks/Dockerfile" || b.BuildArgs["NODE_ENV"] != "production" || b.BuildArgs["VITE_API"] != "https://api" {
		t.Errorf("build request %+v", b)
	}
	if p.env["VITE_API"] != "https://api" {
		t.Errorf("planner got %v", p.env)
	}
	for _, w := range []string{"Generating a build plan with Nixpacks", "out: ║ setup │ nodejs_18 ║", "Building image localhost/bakery/whoami:2 from .nixpacks/Dockerfile"} {
		if !strings.Contains(s.logs.text(), w) {
			t.Errorf("log lacks %q:\n%s", w, s.logs.text())
		}
	}
}

func TestDeployMountsPersistentStorage(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.app = func(a *Application) { a.Storages = []Storage{{Name: "data", MountPath: "/data"}} }
	if _, err := s.service.Deploy(ctx, 1); err != nil {
		t.Fatal(err)
	}
	s.worker.RunOnce(ctx)
	if m := s.runtime.specs[0].Mounts; len(m) != 1 || m[0] != (Mount{Volume: "bakery-app-1-data", Path: "/data"}) {
		t.Fatalf("mounts %+v", m)
	}
	if !strings.Contains(s.logs.text(), "Mounting persistent storage data at /data") {
		t.Fatalf("log:\n%s", s.logs.text())
	}
}

func TestDeployAppliesResourceLimits(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	s.app = func(a *Application) { a.MemoryMB, a.CPUs = 256, 0.5 }
	if _, err := s.service.Deploy(ctx, 1); err != nil {
		t.Fatal(err)
	}
	s.worker.RunOnce(ctx)
	if sp := s.runtime.specs[0]; sp.MemoryMB != 256 || sp.CPUs != 0.5 {
		t.Fatalf("spec %+v", sp)
	}
	if !strings.Contains(s.logs.text(), "Limits: 256 MB memory, 0.5 CPU") {
		t.Fatalf("log:\n%s", s.logs.text())
	}
}

func TestDeployOnTargetServer(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeSource{})
	remote := &fakeRuntime{running: map[string]bool{}}
	s.servers[7] = remote
	s.app = func(a *Application) { a.ServerID = 7 }

	d, err := s.service.Deploy(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.worker.RunOnce(ctx)
	got, _ := s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Finished || got.ServerID != 7 {
		t.Fatalf("%s on %d: %s", got.Status, got.ServerID, got.Error)
	}
	if len(remote.builds) != 1 || len(remote.specs) != 1 || len(s.runtime.builds) != 0 || len(s.runtime.specs) != 0 {
		t.Fatalf("remote built %d started %d, local built %d started %d", len(remote.builds), len(remote.specs), len(s.runtime.builds), len(s.runtime.specs))
	}
	if s.routedOn["whoami.localhost"] != 7 {
		t.Fatalf("routed on %d", s.routedOn["whoami.localhost"])
	}

	// A Server that cannot be reached fails the Deployment before anything
	// runs or moves.
	s.app = func(a *Application) { a.ServerID = 9 }
	delete(s.routes, "whoami.localhost")
	d, err = s.service.Deploy(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.worker.RunOnce(ctx)
	got, _ = s.service.Deployment(ctx, d.ID)
	if got.Status != domain.Failed || got.ServerID != 9 || !strings.Contains(got.Error, "not reachable") {
		t.Fatalf("%s on %d: %q", got.Status, got.ServerID, got.Error)
	}
	if len(s.routes) != 0 || len(remote.builds) != 1 {
		t.Fatalf("routes %v, remote builds %d", s.routes, len(remote.builds))
	}
}

func TestFinishedHearsFinishedAndFailedNotCancelled(t *testing.T) {
	ctx := context.Background()
	type heard struct {
		status domain.Status
		reason string
		slug   string
	}
	listen := func(s *setup) *[]heard {
		var got []heard
		s.worker.Finished = func(_ context.Context, d domain.Deployment, slug string) {
			got = append(got, heard{d.Status, d.Error, slug})
		}
		return &got
	}

	s := newSetup(t, fakeSource{})
	got := listen(s)
	_, _ = s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	s.runtime.buildErr = errors.New("exit status 1")
	_, _ = s.service.Deploy(ctx, 1)
	s.worker.RunOnce(ctx)
	want := []heard{{domain.Finished, "", "whoami"}, {domain.Failed, "build failed: exit status 1", "whoami"}}
	if len(*got) != 2 || (*got)[0] != want[0] || (*got)[1] != want[1] {
		t.Fatalf("heard %+v, want %+v", *got, want)
	}

	c := newSetup(t, fakeSource{})
	got = listen(c)
	c.runtime.building = make(chan struct{})
	d, _ := c.service.Deploy(ctx, 1)
	done := make(chan struct{})
	go func() {
		c.worker.RunOnce(ctx)
		close(done)
	}()
	<-c.runtime.building
	_, _ = c.service.Cancel(ctx, d.ID)
	<-done
	if len(*got) != 0 {
		t.Fatalf("a cancelled deployment was heard: %+v", *got)
	}
}
