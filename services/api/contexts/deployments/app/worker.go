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
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// Worker claims queued Deployments one at a time and runs them.
type Worker struct {
	service  *Service
	cloner   Cloner
	runtimes Runtimes
	router   Router
	// PreviewRouter routes Preview Deployments; nil fails them.
	PreviewRouter PreviewRouter
	// Planner writes the Dockerfile of nixpacks Applications; nil fails them.
	Planner Planner
	// WorkDir holds the clones while they are built.
	WorkDir string
	// Timeout bounds one whole Deployment.
	Timeout time.Duration
	// Poll is how often the queue is checked without a wake-up.
	Poll time.Duration
	Log  func(format string, args ...any)
	// Finished, when set, hears of every Deployment that ended finished or
	// failed (not cancelled), after it was saved, with the Application's
	// Slug (empty when the Application could not be read).
	Finished func(ctx context.Context, d domain.Deployment, slug string)
	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
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

func NewWorker(service *Service, cloner Cloner, runtimes Runtimes, router Router, workDir string) *Worker {
	// Rollbacks and Image retention reach the Servers the same way.
	service.runtimes = runtimes
	return &Worker{
		service: service, cloner: cloner, runtimes: runtimes, router: router, WorkDir: workDir,
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

	var slug string
	err := w.steps(ctx, &d, &slug, log, info)
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
	if d.Preview != 0 && (d.Status == domain.Finished || d.Status == domain.Failed) {
		w.commentPreview(saveCtx, d, info)
	}
	if w.Finished != nil && (d.Status == domain.Finished || d.Status == domain.Failed) {
		w.Finished(saveCtx, d, slug)
	}
}

func (w *Worker) steps(ctx context.Context, d *domain.Deployment, slug *string, log LogWriter, info func(string, ...any)) error {
	app, err := w.service.applications(ctx, d.ApplicationID)
	if err != nil {
		return fmt.Errorf("reading the application: %w", err)
	}
	*slug = app.Slug
	branch := app.GitBranch
	if d.Preview != 0 {
		if app.BuildPack == BuildPackDockerImage {
			return ErrNoPreviews
		}
		p, found, err := w.service.previews.ByNumber(ctx, d.ApplicationID, d.Preview)
		if err != nil {
			return fmt.Errorf("reading the preview: %w", err)
		}
		if !found || p.State != domain.PreviewOpen {
			return domain.ErrPreviewClosed
		}
		branch = p.Branch
	}

	// Every step runs on the Target server. Saved with the next step, or
	// with the failure when the Server cannot be reached.
	d.ServerID = app.ServerID
	rt, err := w.runtimes(ctx, app.ServerID)
	if err != nil {
		return err
	}
	info("Deploying on server %s", rt.ServerName())
	if d.Preview != 0 {
		info("Deploying the preview of pull request #%d (%s)", d.Preview, branch)
	}

	if d.RollbackOf != nil {
		from := "commit " + shortSHA(d.CommitSHA)
		if d.SourceImage != "" {
			from = "pulled " + d.SourceImage
		}
		if d.Trigger == domain.TriggerRestart {
			info("Restarting deployment %d without rebuilding (image %s, %s)", *d.RollbackOf, d.Image, from)
		} else {
			info("Rolling back to deployment %d (image %s, %s)", *d.RollbackOf, d.Image, from)
		}
		ok, err := rt.ImageExists(ctx, d.Image)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w (%s)", ErrImageGone, d.Image)
		}
		return w.goLive(ctx, rt, d, app, info)
	}

	if app.BuildPack == BuildPackDockerImage {
		return w.pull(ctx, rt, d, app, log, info)
	}

	// Clone.
	dir := filepath.Join(w.WorkDir, fmt.Sprintf("deployment-%d", d.ID))
	_ = os.RemoveAll(dir)
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(w.WorkDir, 0o700); err != nil {
		return err
	}
	// Saved now, so the dashboard shows the branch while cloning.
	d.Branch = branch
	if err := w.service.store.Save(ctx, *d); err != nil {
		return err
	}
	info("Cloning %s (branch %s)", app.GitURL, branch)
	commit, err := w.cloner.Clone(ctx, CloneRequest{URL: app.GitURL, Branch: branch, Dir: dir, DeployKey: app.DeployKey}, log.Line)
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
	if d.Preview != 0 {
		labels["bakery.preview"] = fmt.Sprint(d.Preview)
	}
	if len(buildArgs) > 0 {
		info("Build args: %s", strings.Join(slices.Sorted(maps.Keys(buildArgs)), ", "))
	}
	if d.ForceRebuild {
		info("Building without the build cache")
	}
	req := BuildRequest{Dir: dir, Dockerfile: dockerfile, Tag: d.Image, Labels: labels, BuildArgs: buildArgs, NoCache: d.ForceRebuild}
	if err := rt.Build(ctx, req, func(line string) { log.Line(domain.StreamOut, line) }); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return w.goLive(ctx, rt, d, app, info)
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
		return w.Planner.Plan(ctx, dir, app.BuildVariables, log.Line)
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
		if len(app.BuildVariables) > 0 {
			info("Build variables are not used by the static build pack")
		}
		return name, nil, nil
	case "", BuildPackDockerfile:
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(app.DockerfilePath))); err != nil {
			return "", nil, fmt.Errorf("no %s in the repository at %s", app.DockerfilePath, shortSHA(commit.SHA))
		}
		return app.DockerfilePath, app.BuildVariables, nil
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

// pull gets a dockerimage Application's Docker image from its registry and
// tags it as the Deployment's Image; there is nothing to clone or build.
func (w *Worker) pull(ctx context.Context, rt Runtime, d *domain.Deployment, app Application, log LogWriter, info func(string, ...any)) error {
	if err := w.advance(ctx, d, domain.Building); err != nil {
		return err
	}
	d.Image = domain.ImageTag(app.Slug, d.ID)
	if app.RegistryUsername != "" {
		info("Pulling %s with registry credentials for %s", app.DockerImage, app.RegistryUsername)
	} else {
		info("Pulling %s", app.DockerImage)
	}
	digest, err := rt.Pull(ctx, PullRequest{
		Reference: app.DockerImage, Tag: d.Image, Username: app.RegistryUsername, Password: app.RegistryPassword,
	}, func(line string) { log.Line(domain.StreamOut, line) })
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	d.SourceImage = digest
	info("Pulled %s as %s", digest, d.Image)
	if err := w.service.store.Save(ctx, *d); err != nil {
		return err
	}
	return w.goLive(ctx, rt, d, app, info)
}

// limit is what, or "unlimited" when not set.
func limit(set bool, what string) string {
	if !set {
		return "unlimited"
	}
	return what
}

// goLive starts the Deployment's Image, waits for it to be healthy, moves
// the Route to it and removes the Containers it replaces.
func (w *Worker) goLive(ctx context.Context, rt Runtime, d *domain.Deployment, app Application, info func(string, ...any)) error {
	// Start.
	if err := w.advance(ctx, d, domain.Starting); err != nil {
		return err
	}
	d.Container = domain.DeploymentContainerName(*d)
	mounts := make([]Mount, len(app.Storages))
	for i, s := range app.Storages {
		// A Preview never writes into the Application's own Volumes.
		volume := domain.VolumeName(app.ID, s.Name)
		if d.Preview != 0 {
			volume = domain.PreviewVolumeName(app.ID, d.Preview, s.Name)
		}
		mounts[i] = Mount{Volume: volume, Path: s.MountPath}
		info("Mounting persistent storage %s at %s", s.Name, s.MountPath)
	}
	if app.MemoryMB != 0 || app.CPUs != 0 {
		info("Limits: %s memory, %s CPU", limit(app.MemoryMB != 0, fmt.Sprintf("%d MB", app.MemoryMB)), limit(app.CPUs != 0, strconv.FormatFloat(app.CPUs, 'f', -1, 64)))
	}
	info("Starting container %s", d.Container)
	if err := rt.Start(ctx, ContainerSpec{
		Name: d.Container, Image: d.Image, ApplicationID: app.ID, DeploymentID: d.ID, Preview: d.Preview, Env: app.RuntimeVariables,
		Mounts: mounts, MemoryMB: app.MemoryMB, CPUs: app.CPUs, Settle: !app.HealthCheck.Enabled,
	}); err != nil {
		return fmt.Errorf("container did not start: %w", err)
	}
	if app.HealthCheck.Enabled {
		if err := w.waitHealthy(ctx, rt, d.Container, app.Port, app.HealthCheck, info); err != nil {
			// The old Container still serves; only the new one goes.
			_ = rt.Remove(context.WithoutCancel(ctx), d.Container)
			return err
		}
	}

	// A Preview closed while it was building must not get its route back.
	if d.Preview != 0 {
		p, found, err := w.service.previews.ByNumber(ctx, d.ApplicationID, d.Preview)
		if err == nil && (!found || p.State != domain.PreviewOpen) {
			err = domain.ErrPreviewClosed
		}
		if err != nil {
			_ = rt.Remove(context.WithoutCancel(ctx), d.Container)
			return err
		}
	}

	// From here on the Deployment is no longer cancellable: the Route
	// moves and the old Container goes, whatever happens to ctx.
	w.service.release(d.ID)
	if ctx.Err() != nil {
		_ = rt.Remove(context.WithoutCancel(ctx), d.Container)
		return ctx.Err()
	}
	ctx = context.WithoutCancel(ctx)

	// Route, before the old Container goes, so traffic never points at
	// nothing.
	domains := app.Domains
	route := func() error { return w.router(ctx, app.ServerID, app.ID, domains, d.Container, app.Port) }
	if d.Preview != 0 {
		if len(app.Domains) == 0 {
			_ = rt.Remove(ctx, d.Container)
			return errors.New("the application has no domain to put its preview under")
		}
		domains = []string{domain.PreviewDomain(d.Preview, app.Domains[0])}
		route = func() error {
			if w.PreviewRouter == nil {
				return errors.New("previews cannot be routed here")
			}
			return w.PreviewRouter(ctx, app.ServerID, app.ID, d.Preview, domains, d.Container, app.Port)
		}
	}
	info("Routing %s to %s:%d", strings.Join(domains, ", "), d.Container, app.Port)
	if err := route(); err != nil {
		// The old Container still serves; only the new one goes.
		_ = rt.Remove(context.WithoutCancel(ctx), d.Container)
		return fmt.Errorf("routing failed: %w", err)
	}

	// Clean up.
	removed, err := rt.RemoveOthers(ctx, app.ID, d.Preview, d.Container)
	for _, name := range removed {
		info("Removed previous container %s", name)
	}
	if err != nil {
		info("Could not remove every previous container: %v", err)
	}
	return w.advance(ctx, d, domain.Finished)
}

// commentPreview writes the Preview comment for a Preview Deployment that
// ended. A failure goes to the Deployment log; the Deployment stands.
func (w *Worker) commentPreview(ctx context.Context, d domain.Deployment, info func(string, ...any)) {
	c := w.service.Comments
	if c == nil {
		return
	}
	app, err := w.service.applications(ctx, d.ApplicationID)
	if err != nil || len(app.Domains) == 0 {
		return
	}
	posted, err := c.Comment(ctx, d.ApplicationID, d.Preview, c.DeployedBody(d, domain.PreviewDomain(d.Preview, app.Domains[0])))
	switch {
	case err != nil:
		info("Could not comment on pull request #%d: %v", d.Preview, err)
	case posted:
		info("Commented on pull request #%d", d.Preview)
	}
}

// waitHealthy probes the new Container until its Health check passes, the
// retries run out, or probing cannot work at all.
func (w *Worker) waitHealthy(ctx context.Context, rt Runtime, container string, port int, h HealthCheck, info func(string, ...any)) error {
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
		ok, d, err := rt.Probe(ctx, container, url, time.Duration(h.Timeout)*time.Second)
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
