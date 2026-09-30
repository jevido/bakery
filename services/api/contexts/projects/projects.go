// Package projects is what other contexts and the router may use from the
// projects context: its routes, ApplicationForDeploy, Environment, the
// ApplicationDeleted and ApplicationDomainsChanged events and the
// OnProjectDeleting check. Nothing else in contexts/projects is for outside
// use.
package projects

import (
	"context"
	"errors"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/projects/app"
	projectshttp "github.com/jevido/bakery/services/api/contexts/projects/http"
	"github.com/jevido/bakery/services/api/contexts/projects/infra"
)

var service *app.Service

func svc() *app.Service {
	if service == nil {
		cfg := facades.Config()
		service = app.NewService(infra.Store{}, infra.NewDeployKey, cfg.GetString("bakery.domain_suffix", "localhost"), cfg.GetString("bakery.dashboard.domain"))
	}
	return service
}

// ErrNotFound is returned for an Application that does not exist.
var ErrNotFound = app.ErrNotFound

// Routes registers the projects API, all behind identity.Auth.
func Routes(r route.Router) {
	c := projectshttp.NewController(svc())
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Get("/api/projects", c.ListProjects)
		r.Post("/api/projects", c.CreateProject)
		r.Get("/api/projects/{id}", c.ShowProject)
		r.Patch("/api/projects/{id}", c.UpdateProject)
		r.Delete("/api/projects/{id}", c.DeleteProject)
		r.Post("/api/environments/{id}/applications", c.CreateApplication)
		r.Get("/api/applications/{id}", c.ShowApplication)
		r.Patch("/api/applications/{id}", c.UpdateApplication)
		r.Delete("/api/applications/{id}", c.DeleteApplication)
		r.Post("/api/applications/{id}/deploy-key", c.RegenerateDeployKey)
		r.Get("/api/applications/{id}/env", c.ShowEnv)
		r.Put("/api/applications/{id}/env", c.ReplaceEnv)
		r.Get("/api/projects/{id}/variables", c.ShowProjectVariables)
		r.Put("/api/projects/{id}/variables", c.ReplaceProjectVariables)
		r.Get("/api/environments/{id}/variables", c.ShowEnvironmentVariables)
		r.Put("/api/environments/{id}/variables", c.ReplaceEnvironmentVariables)
	})
}

// ApplicationSnapshot is an Application as deployments needs it, taken once
// at the start of a Deployment. Its variables are merged (Application over
// Environment over Project), split by scope and decrypted.
type ApplicationSnapshot struct {
	ID   uint64
	Slug string
	// BuildPack is "dockerfile", "nixpacks", "static" or "image".
	BuildPack string
	// ImageReference is set for the image pack, which has no Source.
	ImageReference string
	// RegistryUsername and RegistryPassword (decrypted) are what the image
	// pack pulls with; both empty means anonymous.
	RegistryUsername string
	RegistryPassword string
	PublishDirectory string
	GitURL           string
	GitBranch        string
	DockerfilePath   string
	Port             int
	// Domains are the Application's Domains, primary first.
	Domains    []string
	BuildEnv   map[string]string
	RuntimeEnv map[string]string
	// DeployKey is the Deploy key's private half (OpenSSH PEM), empty for
	// an https Source.
	DeployKey   string
	HealthCheck HealthCheck
	Storages    []Storage
	// MemoryMB and CPUs are the Resource limits; 0 is unlimited.
	MemoryMB int
	CPUs     float64
}

// Storage is a Persistent storage: the volume Name, mounted at MountPath.
type Storage struct {
	Name      string
	MountPath string
}

// HealthCheck is the Application's Health check; times in seconds.
type HealthCheck struct {
	Enabled     bool
	Path        string
	Interval    int
	Timeout     int
	Retries     int
	StartPeriod int
}

// ApplicationExists reports whether the Application exists, for contexts
// that keep something of their own per Application.
func ApplicationExists(ctx context.Context, id uint64) (bool, error) {
	_, err := svc().Application(ctx, id)
	if errors.Is(err, app.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// ApplicationForDeploy returns the snapshot, or ErrNotFound.
func ApplicationForDeploy(ctx context.Context, id uint64) (ApplicationSnapshot, error) {
	a, err := svc().Application(ctx, id)
	if err != nil {
		return ApplicationSnapshot{}, err
	}
	build, runtime, err := svc().MergedVariables(ctx, a)
	if err != nil {
		return ApplicationSnapshot{}, err
	}
	storages := make([]Storage, len(a.Storages))
	for i, s := range a.Storages {
		storages[i] = Storage(s)
	}
	return ApplicationSnapshot{
		ID: a.ID, Slug: a.Slug, BuildPack: string(a.BuildPack), ImageReference: a.ImageReference, PublishDirectory: a.PublishDirectory,
		RegistryUsername: a.RegistryCredentials.Username, RegistryPassword: a.RegistryCredentials.Password,
		GitURL: a.GitURL, GitBranch: a.GitBranch,
		DockerfilePath: a.DockerfilePath, Port: a.Port, Domains: a.Domains, BuildEnv: build, RuntimeEnv: runtime, DeployKey: a.DeployKey.Private,
		HealthCheck: HealthCheck(a.HealthCheck), Storages: storages,
		MemoryMB: a.ResourceLimits.MemoryMB, CPUs: a.ResourceLimits.CPUs,
	}, nil
}

// OnApplicationDomainsChanged registers a handler for the
// ApplicationDomainsChanged event: an update changed the Application's
// Domains, which are passed in their new order.
func OnApplicationDomainsChanged(f func(ctx context.Context, applicationID uint64, domains []string)) {
	svc().OnApplicationDomainsChanged(f)
}

// EnvironmentSnapshot is an Environment as other contexts see it.
type EnvironmentSnapshot struct {
	ID        uint64
	ProjectID uint64
	Name      string
}

// Environment returns the Environment, or ErrNotFound.
func Environment(ctx context.Context, id uint64) (EnvironmentSnapshot, error) {
	e, err := svc().Environment(ctx, id)
	if err != nil {
		return EnvironmentSnapshot{}, err
	}
	return EnvironmentSnapshot{ID: e.ID, ProjectID: e.ProjectID, Name: e.Name}, nil
}

// OnProjectDeleting registers a check asked before a Project is deleted: a
// context that still keeps something in the Project answers true, and the
// deletion is refused with the same error as for a Project with
// Applications. An error aborts the deletion.
func OnProjectDeleting(inUse func(ctx context.Context, projectID uint64) (bool, error)) {
	svc().OnProjectDeleting(inUse)
}

// DomainInUse reports whether an Application has the Domain, or it is the
// dashboard domain. services asks it before giving a Component a Domain.
func DomainInUse(ctx context.Context, domain string) (bool, error) {
	return svc().DomainInUse(ctx, domain)
}

// OnDomainCheck registers a check asked for every Domain an Application is
// given; a context that serves the Domain itself answers true and the
// Application is refused it. An error aborts the create or update.
func OnDomainCheck(inUse func(ctx context.Context, domain string) (bool, error)) {
	svc().OnDomainCheck(inUse)
}

// OnApplicationDeleted registers a handler for the ApplicationDeleted event.
func OnApplicationDeleted(f func(ctx context.Context, applicationID uint64)) {
	svc().OnApplicationDeleted(f)
}
