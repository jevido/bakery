// Package projects is what other contexts and the router may use from the
// projects context: its routes, ApplicationForDeploy (with the Target
// server), Environment, ProjectInGuild, EnvironmentInGuild and
// ApplicationInGuild, the ApplicationDeleted and ApplicationDomainsChanged
// events and the OnProjectDeleting and OnEnvironmentDeleting checks. Nothing
// else in contexts/projects is for outside use.
package projects

import (
	"context"
	"errors"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/projects/app"
	projectshttp "github.com/jevido/bakery/services/api/contexts/projects/http"
	"github.com/jevido/bakery/services/api/contexts/projects/infra"
	"github.com/jevido/bakery/services/api/contexts/servers"
)

var service *app.Service

func svc() *app.Service {
	if service == nil {
		cfg := facades.Config()
		service = app.NewService(infra.Store{}, infra.NewDeployKey, cfg.GetString("bakery.domain_suffix", "localhost"), cfg.GetString("bakery.dashboard.domain"))
		service.ServerUsable = servers.UsableBy
		service.LocalServer = servers.LocalID
		servers.OnServerDeleting(infra.Store{}.ServerInUse)
		servers.OnContainerOwner("application", ApplicationInGuild)
		guilds.OnGuildDeleting("projects", func(ctx context.Context, guildID uint64) (bool, error) {
			ps, err := service.Projects(app.InGuild(ctx, guildID))
			return len(ps) > 0, err
		})
	}
	return service
}

// ErrNotFound is returned for an Application that does not exist.
var ErrNotFound = app.ErrNotFound

// Routes registers the projects API, all behind guilds.Auth; changes need
// manage_applications, reading variables see_secrets.
func Routes(r route.Router) {
	c := projectshttp.NewController(svc(), servers.LocalID, guilds.Current)
	r.Middleware(guilds.Auth).Group(func(r route.Router) {
		r.Get("/api/projects", c.ListProjects)
		r.Get("/api/projects/{id}", c.ShowProject)
		r.Get("/api/environments/{id}", c.ShowEnvironment)
		r.Get("/api/applications/{id}", c.ShowApplication)
	})
	r.Middleware(guilds.Auth, guilds.Can("manage_applications")).Group(func(r route.Router) {
		r.Post("/api/projects", c.CreateProject)
		r.Patch("/api/projects/{id}", c.UpdateProject)
		r.Delete("/api/projects/{id}", c.DeleteProject)
		r.Post("/api/projects/{id}/environments", c.CreateEnvironment)
		r.Patch("/api/environments/{id}", c.UpdateEnvironment)
		r.Delete("/api/environments/{id}", c.DeleteEnvironment)
		r.Post("/api/environments/{id}/applications", c.CreateApplication)
		r.Patch("/api/applications/{id}", c.UpdateApplication)
		r.Delete("/api/applications/{id}", c.DeleteApplication)
		r.Post("/api/applications/{id}/deploy-key", c.RegenerateDeployKey)
		r.Put("/api/applications/{id}/environment-variables", c.ReplaceEnvironmentVariables)
		r.Put("/api/projects/{id}/variables", c.ReplaceProjectSharedVariables)
		r.Put("/api/environments/{id}/variables", c.ReplaceEnvironmentSharedVariables)
	})
	// Variable values are Secrets.
	r.Middleware(guilds.Auth, guilds.Can("see_secrets")).Group(func(r route.Router) {
		r.Get("/api/applications/{id}/environment-variables", c.ShowEnvironmentVariables)
		r.Get("/api/projects/{id}/variables", c.ShowProjectSharedVariables)
		r.Get("/api/environments/{id}/variables", c.ShowEnvironmentSharedVariables)
	})
}

// ApplicationSnapshot is an Application as deployments needs it, taken once
// at the start of a Deployment. Its variables are merged (Application over
// Environment over Project), split by scope and decrypted.
type ApplicationSnapshot struct {
	ID uint64
	// GuildID is the Guild the Application belongs to (through its Project).
	GuildID uint64
	Slug    string
	// BuildPack is "dockerfile", "nixpacks", "static" or "dockerimage".
	BuildPack string
	// DockerImage is set for the dockerimage pack, which has no Git repository.
	DockerImage string
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
	Domains          []string
	BuildVariables   map[string]string
	RuntimeVariables map[string]string
	// DeployKey is the Deploy key's private half (OpenSSH PEM), empty for
	// an https Git repository.
	DeployKey   string
	HealthCheck HealthCheck
	Storages    []Storage
	// MemoryMB and CPUs are the Resource limits; 0 is unlimited.
	MemoryMB int
	CPUs     float64
	// ServerID is the Target server, 0 for the Local server.
	ServerID uint64
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
		ID: a.ID, GuildID: a.GuildID, Slug: a.Slug, BuildPack: string(a.BuildPack), DockerImage: a.DockerImage, PublishDirectory: a.PublishDirectory,
		RegistryUsername: a.RegistryCredentials.Username, RegistryPassword: a.RegistryCredentials.Password,
		GitURL: a.GitURL, GitBranch: a.GitBranch,
		DockerfilePath: a.DockerfilePath, Port: a.Port, Domains: a.Domains, BuildVariables: build, RuntimeVariables: runtime, DeployKey: a.DeployKey.Private,
		HealthCheck: HealthCheck(a.HealthCheck), Storages: storages,
		MemoryMB: a.ResourceLimits.MemoryMB, CPUs: a.ResourceLimits.CPUs,
		ServerID: a.ServerID,
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
	GuildID   uint64
	Name      string
}

// Environment returns the Environment, or ErrNotFound.
func Environment(ctx context.Context, id uint64) (EnvironmentSnapshot, error) {
	e, err := svc().Environment(ctx, id)
	if err != nil {
		return EnvironmentSnapshot{}, err
	}
	return EnvironmentSnapshot{ID: e.ID, ProjectID: e.ProjectID, GuildID: e.GuildID, Name: e.Name}, nil
}

// ProjectInGuild reports whether the Project exists and belongs to the
// Guild, for the routes of other contexts keyed by a Project id.
func ProjectInGuild(ctx context.Context, projectID, guildID uint64) (bool, error) {
	return found(svc().Project(app.InGuild(ctx, guildID), projectID))
}

// EnvironmentInGuild reports whether the Environment exists and belongs to
// the Guild, for the routes of other contexts keyed by an Environment id.
func EnvironmentInGuild(ctx context.Context, environmentID, guildID uint64) (bool, error) {
	return found(svc().Environment(app.InGuild(ctx, guildID), environmentID))
}

// ApplicationInGuild reports whether the Application exists and belongs to
// the Guild, for the routes of other contexts keyed by an Application id.
func ApplicationInGuild(ctx context.Context, applicationID, guildID uint64) (bool, error) {
	return found(svc().Application(app.InGuild(ctx, guildID), applicationID))
}

func found[T any](_ T, err error) (bool, error) {
	if errors.Is(err, app.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// OnProjectDeleting registers a check asked before a Project is deleted: a
// context that still keeps something in the Project answers true, and the
// deletion is refused with the same error as for a Project with
// Applications. An error aborts the deletion.
func OnProjectDeleting(inUse func(ctx context.Context, projectID uint64) (bool, error)) {
	svc().OnProjectDeleting(inUse)
}

// OnEnvironmentDeleting registers a check asked before an Environment is
// deleted: a context that still keeps something in the Environment answers
// true, and the deletion is refused as for an Environment with
// Applications. An error aborts the deletion.
func OnEnvironmentDeleting(inUse func(ctx context.Context, environmentID uint64) (bool, error)) {
	svc().OnEnvironmentDeleting(inUse)
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

// ApplicationDeleted is the event published when an Application is deleted.
type ApplicationDeleted = app.ApplicationDeleted

// OnApplicationDeleted registers a handler for the ApplicationDeleted event.
func OnApplicationDeleted(f func(ctx context.Context, e ApplicationDeleted)) {
	svc().OnApplicationDeleted(f)
}
