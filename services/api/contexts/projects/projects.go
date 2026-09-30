// Package projects is what other contexts and the router may use from the
// projects context: its routes, ApplicationForDeploy and the
// ApplicationDeleted event. Nothing else in contexts/projects is for outside
// use.
package projects

import (
	"context"

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
	return ApplicationSnapshot{
		ID: a.ID, Slug: a.Slug, BuildPack: string(a.BuildPack), ImageReference: a.ImageReference, PublishDirectory: a.PublishDirectory,
		RegistryUsername: a.RegistryCredentials.Username, RegistryPassword: a.RegistryCredentials.Password,
		GitURL: a.GitURL, GitBranch: a.GitBranch,
		DockerfilePath: a.DockerfilePath, Port: a.Port, Domains: a.Domains, BuildEnv: build, RuntimeEnv: runtime, DeployKey: a.DeployKey.Private,
		HealthCheck: HealthCheck(a.HealthCheck),
	}, nil
}

// OnApplicationDomainsChanged registers a handler for the
// ApplicationDomainsChanged event: an update changed the Application's
// Domains, which are passed in their new order.
func OnApplicationDomainsChanged(f func(ctx context.Context, applicationID uint64, domains []string)) {
	svc().OnApplicationDomainsChanged(f)
}

// OnApplicationDeleted registers a handler for the ApplicationDeleted event.
func OnApplicationDeleted(f func(ctx context.Context, applicationID uint64)) {
	svc().OnApplicationDeleted(f)
}
