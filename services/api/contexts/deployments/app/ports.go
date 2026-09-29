// Package app holds the deployments use cases: queue a Deployment, and the
// Worker that runs queued ones step by step.
package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

var ErrNotFound = errors.New("not found")

// Store keeps Deployments.
type Store interface {
	// Queue adds a queued Deployment, or returns domain.ErrAlreadyQueued
	// when the Application already has a queued one.
	Queue(ctx context.Context, applicationID uint64, trigger domain.Trigger) (domain.Deployment, error)
	// ClaimNext moves the oldest queued Deployment whose Application has no
	// running one to cloning and returns it; concurrent workers never claim
	// the same one.
	ClaimNext(ctx context.Context) (domain.Deployment, bool, error)
	// Save writes status, branch, commit, image, container, error and the
	// times.
	Save(ctx context.Context, d domain.Deployment) error
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
	Env            map[string]string
	// DeployKey is the private key an SSH Source is cloned with; empty for
	// https.
	DeployKey string
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
	Build(ctx context.Context, dir, dockerfile, tag string, labels map[string]string, out func(line string)) error
	// Start creates and starts the Container and returns once it has
	// stayed running, or an error (the Container is then removed).
	Start(ctx context.Context, spec ContainerSpec) error
	// RemoveOthers removes the Application's Containers except keep, and
	// returns the names removed.
	RemoveOthers(ctx context.Context, applicationID uint64, keep string) ([]string, error)
	// Remove removes one Container.
	Remove(ctx context.Context, name string) error
	// RemoveAll removes every Container of the Application.
	RemoveAll(ctx context.Context, applicationID uint64) error
}

type ContainerSpec struct {
	Name          string
	Image         string
	ApplicationID uint64
	DeploymentID  uint64
	Env           map[string]string
}

// Router is routing's SwitchRoute.
type Router func(ctx context.Context, applicationID uint64, domain, container string, port int) error
