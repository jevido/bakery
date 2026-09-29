package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// Worker claims queued Deployments one at a time and runs them.
type Worker struct {
	service *Service
	source  Source
	runtime Runtime
	router  Router
	// WorkDir holds the clones while they are built.
	WorkDir string
	// Timeout bounds one whole Deployment.
	Timeout time.Duration
	// Poll is how often the queue is checked without a wake-up.
	Poll time.Duration
	Log  func(format string, args ...any)
	now  func() time.Time
}

func NewWorker(service *Service, source Source, runtime Runtime, router Router, workDir string) *Worker {
	return &Worker{
		service: service, source: source, runtime: runtime, router: router, WorkDir: workDir,
		Timeout: 30 * time.Minute, Poll: 2 * time.Second, Log: func(string, ...any) {}, now: time.Now,
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
	ctx, cancel := context.WithTimeout(parent, w.Timeout)
	defer cancel()
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
	if err != nil {
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

	// Clone.
	dir := filepath.Join(w.WorkDir, fmt.Sprintf("deployment-%d", d.ID))
	_ = os.RemoveAll(dir)
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(w.WorkDir, 0o700); err != nil {
		return err
	}
	info("Cloning %s (branch %s)", app.GitURL, app.GitBranch)
	commit, err := w.source.Clone(ctx, app.GitURL, app.GitBranch, dir, log.Line)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}
	d.CommitSHA = commit
	info("Checked out commit %s", commit)

	// Build.
	if err := w.advance(ctx, d, domain.Building); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(app.DockerfilePath))); err != nil {
		return fmt.Errorf("no %s in the repository at %s", app.DockerfilePath, shortSHA(commit))
	}
	d.Image = domain.ImageTag(app.Slug, d.ID)
	info("Building image %s from %s", d.Image, app.DockerfilePath)
	labels := map[string]string{"bakery.managed": "true", "bakery.application": fmt.Sprint(app.ID), "bakery.deployment": fmt.Sprint(d.ID)}
	if err := w.runtime.Build(ctx, dir, app.DockerfilePath, d.Image, labels, func(line string) { log.Line(domain.StreamOut, line) }); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Start.
	if err := w.advance(ctx, d, domain.Starting); err != nil {
		return err
	}
	d.Container = domain.ContainerName(app.ID, d.ID)
	info("Starting container %s", d.Container)
	if err := w.runtime.Start(ctx, ContainerSpec{
		Name: d.Container, Image: d.Image, ApplicationID: app.ID, DeploymentID: d.ID, Env: app.Env,
	}); err != nil {
		return fmt.Errorf("container did not start: %w", err)
	}

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
