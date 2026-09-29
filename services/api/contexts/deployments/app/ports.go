// Package app holds the deployments use cases: queue a Deployment, and the
// Worker that runs queued ones step by step.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

var (
	ErrNotFound = errors.New("not found")
	// ErrNotCancellable is a Cancel of a Deployment that has ended, or whose
	// Route has already moved.
	ErrNotCancellable = errors.New("deployment can no longer be cancelled")
	// ErrImageGone is a Rollback to a Deployment whose Image was removed.
	ErrImageGone = errors.New("the image of that deployment is gone")
)

// Store keeps Deployments.
type Store interface {
	// Queue stores a new queued Deployment, or returns
	// domain.ErrAlreadyQueued when the Application already has a queued one.
	Queue(ctx context.Context, d domain.Deployment) (domain.Deployment, error)
	// ClaimNext moves the oldest queued Deployment whose Application has no
	// running one to cloning and returns it; concurrent workers never claim
	// the same one.
	ClaimNext(ctx context.Context) (domain.Deployment, bool, error)
	// Save writes status, branch, commit, image, container, error and the
	// times.
	Save(ctx context.Context, d domain.Deployment) error
	// CancelQueued cancels the Deployment if it is still queued, reporting
	// whether it was; a Worker claiming it at the same moment wins or loses
	// cleanly.
	CancelQueued(ctx context.Context, id uint64) (bool, error)
	// FailInterrupted fails every Deployment left active by a crash or
	// restart (not queued ones, which still run) and returns how many.
	FailInterrupted(ctx context.Context, reason string) (int, error)
	ByID(ctx context.Context, id uint64) (domain.Deployment, bool, error)
	ByApplication(ctx context.Context, applicationID uint64, limit int) ([]domain.Deployment, error)
	// Active returns the Application's active Deployment, if any.
	Active(ctx context.Context, applicationID uint64) (domain.Deployment, bool, error)
	DeleteForApplication(ctx context.Context, applicationID uint64) error
}

// Logs writes and reads Deployment logs.
type Logs interface {
	// Writer returns a writer for one Deployment's log. Close flushes it.
	Writer(deploymentID uint64) LogWriter
	After(ctx context.Context, deploymentID, afterID uint64, limit int) ([]domain.LogLine, error)
}

type LogWriter interface {
	Line(stream, line string)
	Close() error
}

// Application is what a Deployment needs to know about its Application,
// snapshotted once at the start.
type Application struct {
	ID             uint64
	Slug           string
	GitURL         string
	GitBranch      string
	DockerfilePath string
	Port           int
	Domain         string
	// BuildEnv reaches the build as build args, RuntimeEnv the Container.
	BuildEnv   map[string]string
	RuntimeEnv map[string]string
	// DeployKey is the private key an SSH Source is cloned with; empty for
	// https.
	DeployKey   string
	HealthCheck HealthCheck
}

// HealthCheck is how the new Container is probed before the Route moves to
// it. Times are in seconds.
type HealthCheck struct {
	Enabled     bool
	Path        string
	Interval    int
	Timeout     int
	Retries     int
	StartPeriod int
}

// Applications is projects' published ApplicationForDeploy.
type Applications func(ctx context.Context, id uint64) (Application, error)

// Commit is the commit a clone checked out.
type Commit struct {
	SHA     string
	Subject string
	Author  string
}

// CloneRequest is what a clone needs. DeployKey is the private key for an
// SSH URL, empty for https.
type CloneRequest struct {
	URL       string
	Branch    string
	Dir       string
	DeployKey string
}

// Source fetches an Application's code.
type Source interface {
	// Clone checks the branch out into Dir (which must not exist) and
	// returns the commit.
	Clone(ctx context.Context, req CloneRequest, out func(stream, line string)) (Commit, error)
}

// KnownHosts keeps the SSH host keys of git hosts.
type KnownHosts interface {
	// Lines returns every stored known_hosts line.
	Lines(ctx context.Context) (string, error)
	// Remember adds known_hosts lines, merged per host.
	Remember(ctx context.Context, lines string) error
	List(ctx context.Context) ([]domain.KnownHost, error)
	// Forget removes a host, reporting whether it existed.
	Forget(ctx context.Context, id uint64) (bool, error)
}

// Runtime builds and runs Containers.
type Runtime interface {
	Build(ctx context.Context, req BuildRequest, out func(line string)) error
	// Start creates and starts the Container and returns once it is
	// running (and, with spec.Settle, has stayed running for a moment), or
	// an error (the Container is then removed).
	Start(ctx context.Context, spec ContainerSpec) error
	// Probe requests url inside the running Container once, within
	// timeout. ok is false with a detail when it did not answer 2xx or 3xx;
	// err means probing cannot work at all (the Container stopped, or the
	// image has no curl or wget), so retrying is pointless.
	Probe(ctx context.Context, container, url string, timeout time.Duration) (ok bool, detail string, err error)
	// RemoveOthers removes the Application's Containers except keep, and
	// returns the names removed.
	RemoveOthers(ctx context.Context, applicationID uint64, keep string) ([]string, error)
	// ImageExists reports whether the Image is still there.
	ImageExists(ctx context.Context, image string) (bool, error)
	// Remove removes one Container.
	Remove(ctx context.Context, name string) error
	// RemoveAll removes every Container of the Application.
	RemoveAll(ctx context.Context, applicationID uint64) error
}

type BuildRequest struct {
	Dir        string
	Dockerfile string
	Tag        string
	Labels     map[string]string
	BuildArgs  map[string]string
}

type ContainerSpec struct {
	Name          string
	Image         string
	ApplicationID uint64
	DeploymentID  uint64
	Env           map[string]string
	// Settle makes Start wait until the Container has stayed running for a
	// moment; used when there is no Health check to wait for instead.
	Settle bool
}

// Router is routing's SwitchRoute.
type Router func(ctx context.Context, applicationID uint64, domain, container string, port int) error
