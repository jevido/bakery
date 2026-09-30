package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// Worker claims queued Deployments one at a time and runs them.
type Worker struct {
	service *Service
	source  Source
	runtime Runtime
	router  Router
	// Planner writes the Dockerfile of nixpacks Applications; nil fails them.
	Planner Planner
	// WorkDir holds the clones while they are built.
	WorkDir string
	// Timeout bounds one whole Deployment.
	Timeout time.Duration
	// Poll is how often the queue is checked without a wake-up.
	Poll  time.Duration
	Log   func(format string, args ...any)
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func NewWorker(service *Service, source Source, runtime Runtime, router Router, workDir string) *Worker {
	// Rollbacks check with the same Runtime that the Image is still there.
	service.images = runtime.ImageExists
	return &Worker{
		service: service, source: source, runtime: runtime, router: router, WorkDir: workDir,
		Timeout: 30 * time.Minute, Poll: 2 * time.Second, Log: func(string, ...any) {}, now: time.Now, sleep: sleep,
	}
}

// Recover fails Deployments a previous process left half done.
func (w *Worker) Recover(ctx context.Context) error {
	n, err := w.service.store.FailInterrupted(ctx, "interrupted by restart")
	if n > 0 {
		w.Log("deployments: failed %d deployment(s) interrupted by a restart", n)
	}
	return err
}

// Run works the queue until ctx ends.
func (w *Worker) Run(ctx context.Context) {
	for {
		for w.RunOnce(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-w.service.wake:
		case <-time.After(w.Poll):
		}
	}
}

// RunOnce claims and runs one Deployment, reporting whether there was one.
func (w *Worker) RunOnce(ctx context.Context) bool {
	d, found, err := w.service.store.ClaimNext(ctx)
	if err != nil {
		w.Log("deployments: claiming: %v", err)
		return false
	}
	if !found {
		return false
	}
	w.run(ctx, d)
	return true
}

func (w *Worker) run(parent context.Context, d domain.Deployment) {
	cancellable, cancelWith := context.WithCancelCause(parent)
	defer cancelWith(nil)
	ctx, cancel := context.WithTimeout(cancellable, w.Timeout)
	defer cancel()
	w.service.register(d.ID, cancelWith)
	defer w.service.release(d.ID)
	log := w.service.logs.Writer(d.ID)
	defer func() {
		if err := log.Close(); err != nil {
			w.Log("deployments: writing log of %d: %v", d.ID, err)
		}
	}()
	info := func(format string, args ...any) { log.Line(domain.StreamInfo, fmt.Sprintf(format, args...)) }
	started := w.now()

	err := w.steps(ctx, &d, log, info)
	// Saving the outcome must not be cut short by the Deployment's own
	// timeout or a shutdown.
	saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
	defer cancelSave()
	end := w.now()
	d.FinishedAt = &end
	if err != nil && errors.Is(context.Cause(ctx), domain.ErrCancelled) {
		info("Deployment cancelled.")
		if cerr := d.Cancel(); cerr != nil {
			w.Log("deployments: %v", cerr)
		}
	} else if err != nil {
		reason := err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			reason = "timed out after " + w.Timeout.String()
		} else if parent.Err() != nil {
			reason = "interrupted by shutdown"
		}
		info("Deployment failed: %s", reason)
		if ferr := d.Fail(reason); ferr != nil {
			w.Log("deployments: %v", ferr)
		}
	} else {
		info("Deployment finished in %s.", end.Sub(started).Round(time.Second))
	}
	if serr := w.service.store.Save(saveCtx, d); serr != nil {
		w.Log("deployments: saving %d: %v", d.ID, serr)
	}
}

func (w *Worker) steps(ctx context.Context, d *domain.Deployment, log LogWriter, info func(string, ...any)) error {
	app, err := w.service.applications(ctx, d.ApplicationID)
	if err != nil {
		return fmt.Errorf("reading the application: %w", err)
	}

	if d.RollbackOf != nil {
		from := "commit " + shortSHA(d.CommitSHA)
		if d.SourceImage != "" {
			from = "pulled " + d.SourceImage
		}
		info("Rolling back to deployment %d (image %s, %s)", *d.RollbackOf, d.Image, from)
		ok, err := w.runtime.ImageExists(ctx, d.Image)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w (%s)", ErrImageGone, d.Image)
		}
		return w.goLive(ctx, d, app, info)
	}

	if app.BuildPack == BuildPackImage {
		return w.pull(ctx, d, app, log, info)
	}

	// Clone.
	dir := filepath.Join(w.WorkDir, fmt.Sprintf("deployment-%d", d.ID))
	_ = os.RemoveAll(dir)
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(w.WorkDir, 0o700); err != nil {
		return err
	}
	// Saved now, so the dashboard shows the branch while cloning.
	d.Branch = app.GitBranch
	if err := w.service.store.Save(ctx, *d); err != nil {
		return err
	}
	info("Cloning %s (branch %s)", app.GitURL, app.GitBranch)
	commit, err := w.source.Clone(ctx, CloneRequest{URL: app.GitURL, Branch: app.GitBranch, Dir: dir, DeployKey: app.DeployKey}, log.Line)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}
	d.CommitSHA, d.CommitMessage, d.CommitAuthor = commit.SHA, commit.Subject, commit.Author
	info("Checked out %s %q by %s", shortSHA(commit.SHA), commit.Subject, commit.Author)

	// Build.
	if err := w.advance(ctx, d, domain.Building); err != nil {
		return err
	}
	dockerfile, buildArgs, err := w.dockerfile(ctx, d, app, dir, commit, log, info)
	if err != nil {
		return err
	}
	d.Image = domain.ImageTag(app.Slug, d.ID)
	info("Building image %s from %s", d.Image, dockerfile)
	labels := map[string]string{"bakery.managed": "true", "bakery.application": fmt.Sprint(app.ID), "bakery.deployment": fmt.Sprint(d.ID)}
	if len(buildArgs) > 0 {
		info("Build args: %s", strings.Join(slices.Sorted(maps.Keys(buildArgs)), ", "))
	}
	req := BuildRequest{Dir: dir, Dockerfile: dockerfile, Tag: d.Image, Labels: labels, BuildArgs: buildArgs}
	if err := w.runtime.Build(ctx, req, func(line string) { log.Line(domain.StreamOut, line) }); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return w.goLive(ctx, d, app, info)
}

// dockerfile returns the Dockerfile to build from the clone in dir, and the
// build args it gets, by Build pack.
func (w *Worker) dockerfile(ctx context.Context, d *domain.Deployment, app Application, dir string, commit Commit, log LogWriter, info func(string, ...any)) (string, map[string]string, error) {
	switch app.BuildPack {
	case BuildPackNixpacks:
		if w.Planner == nil {
			return "", nil, errors.New("the nixpacks build pack is not available here")
		}
		info("Generating a build plan with Nixpacks")
		return w.Planner.Plan(ctx, dir, app.BuildEnv, log.Line)
	case BuildPackStatic:
		publish := path.Clean(app.PublishDirectory)
		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(publish))); err != nil || !st.IsDir() {
			return "", nil, fmt.Errorf("no directory %s in the repository at %s", publish, shortSHA(commit.SHA))
		}
		name := fmt.Sprintf(".bakery-static-%d.Containerfile", d.ID)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(StaticContainerfile(publish, name)), 0o600); err != nil {
			return "", nil, err
		}
		info("Serving %s as a static site", publish)
		if len(app.BuildEnv) > 0 {
			info("Build variables are not used by the static build pack")
		}
		return name, nil, nil
	case "", BuildPackDockerfile:
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(app.DockerfilePath))); err != nil {
			return "", nil, fmt.Errorf("no %s in the repository at %s", app.DockerfilePath, shortSHA(commit.SHA))
		}
		return app.DockerfilePath, app.BuildEnv, nil
	}
	return "", nil, fmt.Errorf("build pack %s is not supported yet", app.BuildPack)
}

// StaticContainerfile serves the publish directory of the build context
// with Caddy (the proxy's image, so usually already pulled) on port 80.
// self is the Containerfile's own name, removed again when the whole
// repository is published.
func StaticContainerfile(publish, self string) string {
	src, _ := json.Marshal([]string{strings.TrimSuffix(publish, "/") + "/", "/srv/"})
	file := "FROM docker.io/library/caddy:2\nCOPY " + string(src) + "\n"
	if publish == "." {
		file += "RUN rm -f /srv/" + self + "\n"
	}
	return file + "EXPOSE 80\n" + `CMD ["caddy", "file-server", "--root", "/srv", "--listen", ":80"]` + "\n"
}

// pull gets an image Application's Image reference from its registry and
// tags it as the Deployment's Image; there is nothing to clone or build.
func (w *Worker) pull(ctx context.Context, d *domain.Deployment, app Application, log LogWriter, info func(string, ...any)) error {
	if err := w.advance(ctx, d, domain.Building); err != nil {
		return err
	}
	d.Image = domain.ImageTag(app.Slug, d.ID)
	if app.RegistryUsername != "" {
		info("Pulling %s with registry credentials for %s", app.ImageReference, app.RegistryUsername)
	} else {
		info("Pulling %s", app.ImageReference)
	}
	digest, err := w.runtime.Pull(ctx, PullRequest{
		Reference: app.ImageReference, Tag: d.Image, Username: app.RegistryUsername, Password: app.RegistryPassword,
	}, func(line string) { log.Line(domain.StreamOut, line) })
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	d.SourceImage = digest
	info("Pulled %s as %s", digest, d.Image)
	if err := w.service.store.Save(ctx, *d); err != nil {
		return err
	}
	return w.goLive(ctx, d, app, info)
}

// goLive starts the Deployment's Image, waits for it to be healthy, moves
// the Route to it and removes the Containers it replaces.
func (w *Worker) goLive(ctx context.Context, d *domain.Deployment, app Application, info func(string, ...any)) error {
	// Start.
	if err := w.advance(ctx, d, domain.Starting); err != nil {
		return err
	}
	d.Container = domain.ContainerName(app.ID, d.ID)
	info("Starting container %s", d.Container)
	if err := w.runtime.Start(ctx, ContainerSpec{
		Name: d.Container, Image: d.Image, ApplicationID: app.ID, DeploymentID: d.ID, Env: app.RuntimeEnv,
		Settle: !app.HealthCheck.Enabled,
	}); err != nil {
		return fmt.Errorf("container did not start: %w", err)
	}
	if app.HealthCheck.Enabled {
		if err := w.waitHealthy(ctx, d.Container, app.Port, app.HealthCheck, info); err != nil {
			// The old Container still serves; only the new one goes.
			_ = w.runtime.Remove(context.WithoutCancel(ctx), d.Container)
			return err
		}
	}

	// From here on the Deployment is no longer cancellable: the Route
	// moves and the old Container goes, whatever happens to ctx.
	w.service.release(d.ID)
	if ctx.Err() != nil {
		_ = w.runtime.Remove(context.WithoutCancel(ctx), d.Container)
		return ctx.Err()
	}
	ctx = context.WithoutCancel(ctx)

	// Route, before the old Container goes, so traffic never points at
	// nothing.
	info("Routing %s to %s:%d", app.Domain, d.Container, app.Port)
	if err := w.router(ctx, app.ID, app.Domain, d.Container, app.Port); err != nil {
		// The old Container still serves; only the new one goes.
		_ = w.runtime.Remove(context.WithoutCancel(ctx), d.Container)
		return fmt.Errorf("routing failed: %w", err)
	}

	// Clean up.
	removed, err := w.runtime.RemoveOthers(ctx, app.ID, d.Container)
	for _, name := range removed {
		info("Removed previous container %s", name)
	}
	if err != nil {
		info("Could not remove every previous container: %v", err)
	}
	return w.advance(ctx, d, domain.Finished)
}

// waitHealthy probes the new Container until its Health check passes, the
// retries run out, or probing cannot work at all.
func (w *Worker) waitHealthy(ctx context.Context, container string, port int, h HealthCheck, info func(string, ...any)) error {
	if h.StartPeriod > 0 {
		info("Waiting %ds before the first health check", h.StartPeriod)
		if err := w.sleep(ctx, time.Duration(h.StartPeriod)*time.Second); err != nil {
			return err
		}
	}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, h.Path)
	detail := ""
	for attempt := 1; attempt <= h.Retries; attempt++ {
		if attempt > 1 {
			if err := w.sleep(ctx, time.Duration(h.Interval)*time.Second); err != nil {
				return err
			}
		}
		info("Waiting for %s (attempt %d/%d)", h.Path, attempt, h.Retries)
		ok, d, err := w.runtime.Probe(ctx, container, url, time.Duration(h.Timeout)*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("health check failed: %w", err)
		}
		if ok {
			info("Healthy after %d attempt(s)", attempt)
			return nil
		}
		detail = d
		info("Not healthy yet: %s", d)
	}
	return fmt.Errorf("health check failed after %d attempts: %s", h.Retries, detail)
}

// advance moves the Deployment on and saves it, so the dashboard sees the
// new status.
func (w *Worker) advance(ctx context.Context, d *domain.Deployment, to domain.Status) error {
	if err := d.Advance(to); err != nil {
		return err
	}
	if to == domain.Finished {
		return nil // saved with its finish time by run
	}
	return w.service.store.Save(ctx, *d)
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
