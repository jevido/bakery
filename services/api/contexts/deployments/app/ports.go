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
	// ApplicationIDs lists every Application that has Deployments.
	ApplicationIDs(ctx context.Context) ([]uint64, error)
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
	ID   uint64
	Slug string
	// BuildPack is one of the BuildPack* constants.
	BuildPack string
	// ImageReference is what the image pack pulls; it has no Source.
	ImageReference string
	// RegistryUsername and RegistryPassword pull ImageReference; both empty
	// is anonymous.
	RegistryUsername string
	RegistryPassword string
	// PublishDirectory is what the static pack serves.
	PublishDirectory string
	GitURL           string
	GitBranch        string
	DockerfilePath   string
	Port             int
	// Domains are the Application's Domains, primary first.
	Domains []string
	// BuildEnv reaches the build as build args, RuntimeEnv the Container.
	BuildEnv   map[string]string
	RuntimeEnv map[string]string
	// DeployKey is the private key an SSH Source is cloned with; empty for
	// https.
	DeployKey   string
	HealthCheck HealthCheck
	// Storages are the Persistent storages every Container mounts.
	Storages []Storage
	// MemoryMB and CPUs are the Resource limits; 0 is unlimited.
	MemoryMB int
	CPUs     float64
}

// Storage is a Persistent storage of the Application.
type Storage struct {
	Name      string
	MountPath string
}

// Build packs, as projects names them.
const (
	BuildPackDockerfile = "dockerfile"
	BuildPackNixpacks   = "nixpacks"
	BuildPackStatic     = "static"
	BuildPackImage      = "image"
)

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

// Planner writes a Dockerfile for a clone that has none (Nixpacks).
type Planner interface {
	// Plan writes the Dockerfile into dir and returns its path relative to
	// dir and the build args to build it with (buildEnv included).
	Plan(ctx context.Context, dir string, buildEnv map[string]string, out func(stream, line string)) (dockerfile string, buildArgs map[string]string, err error)
}

// Runtime builds and runs Containers.
type Runtime interface {
	Build(ctx context.Context, req BuildRequest, out func(line string)) error
	// Pull pulls req.Reference, tags it req.Tag and returns the pulled
	// reference with its digest.
	Pull(ctx context.Context, req PullRequest, out func(line string)) (digest string, err error)
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
	// RemoveImage removes the Image unless a Container uses it, returning
	// the bytes freed (0 when it was only untagged or kept).
	RemoveImage(ctx context.Context, image string) (int64, error)
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

// PullRequest is an image pull. Username and Password empty is anonymous.
type PullRequest struct {
	Reference string
	Tag       string
	Username  string
	Password  string
}

type ContainerSpec struct {
	Name          string
	Image         string
	ApplicationID uint64
	DeploymentID  uint64
	Env           map[string]string
	// Mounts are the Volumes to create (if missing) and mount.
	Mounts []Mount
	// MemoryMB and CPUs limit the Container; 0 is unlimited.
	MemoryMB int
	CPUs     float64
	// Settle makes Start wait until the Container has stayed running for a
	// moment; used when there is no Health check to wait for instead.
	Settle bool
}

// Mount is a Volume mounted at a path in the Container.
type Mount struct {
	Volume string
	Path   string
}

// Router is routing's SwitchRoute.
type Router func(ctx context.Context, applicationID uint64, domains []string, container string, port int) error
