// Package projects is what other contexts and the router may use from the
// projects context: its routes, ApplicationForDeploy (with the Target
// server), Environment, ProjectOf, ApplicationInGuild and
// ApplicationsOnServer, ProjectNames, the ApplicationDeleted,
// ApplicationDomainsChanged and OnProjectDeleted events and the
// OnProjectDeleting and OnEnvironmentDeleting checks. Nothing
// else in contexts/projects is for outside use.
package projects

import (
	"context"
	"errors"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/projects/app"
	"github.com/jevido/bakery/services/api/contexts/projects/domain"
	projectshttp "github.com/jevido/bakery/services/api/contexts/projects/http"
	"github.com/jevido/bakery/services/api/contexts/projects/infra"
	"github.com/jevido/bakery/services/api/contexts/servers"
)

var (
	service        *app.Service
	projectDeleted []func(ctx context.Context, projectID uint64) error
)

func svc() *app.Service {
	if service == nil {
		cfg := facades.Config()
		service = app.NewService(infra.Store{}, infra.NewDeployKey, cfg.GetString("bakery.domain_suffix", "localhost"), cfg.GetString("bakery.dashboard.domain"))
		service.ServerUsable = servers.UsableBy
		service.LocalServer = servers.LocalID
		service.ProjectDeleted = func(ctx context.Context, projectID uint64) error {
			if err := guilds.ForgetProject(ctx, projectID); err != nil {
				return err
			}
			for _, f := range projectDeleted {
				if err := f(ctx, projectID); err != nil {
					return err
				}
			}
			return nil
		}
		servers.OnServerDeleting(infra.Store{}.ServerInUse)
		servers.OnContainerOwner("application", ApplicationInGuild)
		servers.OnResourceProjects(resourceProjects)
		guilds.OnGuildDeleting("projects", func(ctx context.Context, guildID uint64) (bool, error) {
			ps, err := service.Projects(app.InGuild(ctx, guildID))
			return len(ps) > 0, err
		})
	}
	return service
}

// ErrNotFound is returned for an Application that does not exist.
var ErrNotFound = app.ErrNotFound

// Routes registers the projects API, all behind guilds.Auth; a route keyed
// by a Project, Environment or Application also behind guilds.InProject,
// so its Permission overrides count. Changes need manage_applications,
// reading variables see_secrets. A Project's Permission overrides are
// guilds' routes, registered here with ProjectOf. An Agent's Run key may
// list and read Projects (guilds.AuthAgents), nothing more yet.
func Routes(r route.Router) {
	c := projectshttp.NewController(svc(), servers.LocalID, guilds.Current)
	c.Visible = guilds.VisibleProjects
	c.Permissions = guilds.Permissions
	project := guilds.InProject("project", ProjectOf("project"))
	environment := guilds.InProject("environment", ProjectOf("environment"))
	application := guilds.InProject("application", ProjectOf("application"))
	manage, secrets := guilds.Can("manage_applications"), guilds.Can("see_secrets")
	r.Middleware(guilds.AuthAgents).Get("/api/projects", c.ListProjects)
	r.Middleware(guilds.Auth, manage).Post("/api/projects", c.CreateProject)
	r.Middleware(guilds.AuthAgents, project).Get("/api/projects/{id}", c.ShowProject)
	r.Middleware(guilds.Auth, project, manage).Group(func(r route.Router) {
		r.Patch("/api/projects/{id}", c.UpdateProject)
		r.Delete("/api/projects/{id}", c.DeleteProject)
		r.Post("/api/projects/{id}/environments", c.CreateEnvironment)
		r.Put("/api/projects/{id}/variables", c.ReplaceProjectSharedVariables)
	})
	r.Middleware(guilds.Auth, environment).Get("/api/environments/{id}", c.ShowEnvironment)
	r.Middleware(guilds.Auth, environment, manage).Group(func(r route.Router) {
		r.Patch("/api/environments/{id}", c.UpdateEnvironment)
		r.Delete("/api/environments/{id}", c.DeleteEnvironment)
		r.Post("/api/environments/{id}/applications", c.CreateApplication)
		r.Put("/api/environments/{id}/variables", c.ReplaceEnvironmentSharedVariables)
	})
	r.Middleware(guilds.Auth, application).Get("/api/applications/{id}", c.ShowApplication)
	r.Middleware(guilds.Auth, application, manage).Group(func(r route.Router) {
		r.Patch("/api/applications/{id}", c.UpdateApplication)
		r.Delete("/api/applications/{id}", c.DeleteApplication)
		r.Post("/api/applications/{id}/deploy-key", c.RegenerateDeployKey)
		r.Put("/api/applications/{id}/environment-variables", c.ReplaceEnvironmentVariables)
	})
	// Variable values are Secrets.
	r.Middleware(guilds.Auth, project, secrets).Get("/api/projects/{id}/variables", c.ShowProjectSharedVariables)
	r.Middleware(guilds.Auth, environment, secrets).Get("/api/environments/{id}/variables", c.ShowEnvironmentSharedVariables)
	r.Middleware(guilds.Auth, application, secrets).Get("/api/applications/{id}/environment-variables", c.ShowEnvironmentVariables)
	guilds.ProjectPermissionRoutes(r, ProjectOf("project"))
}

// ProjectOf finds the Project and Guild of a "project", "environment" or
// "application" by id, for guilds.InProject on the routes of every context
// keyed by one; another kind panics, at boot where routes are registered.
func ProjectOf(kind string) guilds.ProjectOf {
	var of func(ctx context.Context, id uint64) (projectID, guildID uint64, err error)
	switch kind {
	case "project":
		of = func(ctx context.Context, id uint64) (uint64, uint64, error) {
			p, err := svc().Project(ctx, id)
			return p.ID, p.GuildID, err
		}
	case "environment":
		of = func(ctx context.Context, id uint64) (uint64, uint64, error) {
			e, err := svc().Environment(ctx, id)
			return e.ProjectID, e.GuildID, err
		}
	case "application":
		of = func(ctx context.Context, id uint64) (uint64, uint64, error) {
			a, err := svc().Application(ctx, id)
			return a.ProjectID, a.GuildID, err
		}
	default:
		panic("projects: no project of " + kind)
	}
	return func(ctx context.Context, id uint64) (uint64, uint64, bool, error) {
		projectID, guildID, err := of(ctx, id)
		if errors.Is(err, app.ErrNotFound) {
			return 0, 0, false, nil
		}
		return projectID, guildID, err == nil, err
	}
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

// resourceProjects lists the Guild's Projects and their Environments for a
// Server's Resources list.
func resourceProjects(ctx context.Context, guildID uint64) ([]servers.ResourceProject, error) {
	ps, err := svc().ProjectsWithEnvironments(app.InGuild(ctx, guildID))
	if err != nil {
		return nil, err
	}
	out := make([]servers.ResourceProject, len(ps))
	for i, p := range ps {
		envs := make(map[uint64]string, len(p.Environments))
		for _, e := range p.Environments {
			envs[e.ID] = e.Name
		}
		out[i] = servers.ResourceProject{ID: p.ID, Name: p.Name, Environments: envs}
	}
	return out, nil
}

// ApplicationOnServer is an Application as a Server's Resources list sees
// it.
type ApplicationOnServer struct {
	ID            uint64
	Name          string
	EnvironmentID uint64
}

// ApplicationsOnServer lists the Applications in the Environments that
// target the Server (0 for the Local server).
func ApplicationsOnServer(ctx context.Context, serverID uint64, environmentIDs []uint64) ([]ApplicationOnServer, error) {
	list, err := svc().ApplicationsOnServer(ctx, serverID, environmentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]ApplicationOnServer, len(list))
	for i, a := range list {
		out[i] = ApplicationOnServer{ID: a.ID, Name: a.Name, EnvironmentID: a.EnvironmentID}
	}
	return out, nil
}

// ApplicationInGuild reports whether the Application exists and belongs to
// the Guild, for the routes of other contexts keyed by an Application id.
func ApplicationInGuild(ctx context.Context, applicationID, guildID uint64) (bool, error) {
	return found(svc().Application(app.InGuild(ctx, guildID), applicationID))
}

// ApplicationInProject reports whether the Application exists, belongs to
// the Guild and sits in one of the Project's Environments, for work's
// Issues that name an Application of their Project.
func ApplicationInProject(ctx context.Context, guildID, projectID, applicationID uint64) (bool, error) {
	a, err := svc().Application(app.InGuild(ctx, guildID), applicationID)
	if err != nil {
		return found(a, err)
	}
	return a.ProjectID == projectID, nil
}

// GitRepository is an Application's git source as agents needs it for a
// Run's Workspace: the Application's name, its Git repository URL and its
// branch. It never carries the Deploy key: the laptop pushes with its
// person's own git access.
type GitRepository struct {
	Name   string
	URL    string
	Branch string
}

// ApplicationRepository tells the Guild's Application's git source; found
// is false when it is another Guild's, there is none, or it has no git
// source (the dockerimage build pack).
func ApplicationRepository(ctx context.Context, guildID, applicationID uint64) (GitRepository, bool, error) {
	a, err := svc().Application(app.InGuild(ctx, guildID), applicationID)
	if errors.Is(err, app.ErrNotFound) {
		return GitRepository{}, false, nil
	}
	if err != nil || a.BuildPack == domain.DockerImage || a.GitURL == "" {
		return GitRepository{}, false, err
	}
	return GitRepository{Name: a.Name, URL: a.GitURL, Branch: a.GitBranch}, true, nil
}

// ApplicationNames names the Guild's Applications among ids; an id that is
// not one of the Guild's Applications is left out.
func ApplicationNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	ctx = app.InGuild(ctx, guildID)
	out := map[uint64]string{}
	for _, id := range ids {
		if _, done := out[id]; done {
			continue
		}
		a, err := svc().Application(ctx, id)
		if errors.Is(err, app.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = a.Name
	}
	return out, nil
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

// OnProjectDeleted registers a handler told of each deleted Project, after
// it is gone, so a context that refers to Projects (work's Issues) lets go
// of it. An error fails the request, though the Project stays deleted.
func OnProjectDeleted(f func(ctx context.Context, projectID uint64) error) {
	svc()
	projectDeleted = append(projectDeleted, f)
}

// ProjectNames names the Guild's Projects among ids; an id that is not one
// of the Guild's Projects is left out.
func ProjectNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	ps, err := svc().Projects(app.InGuild(ctx, guildID))
	if err != nil {
		return nil, err
	}
	want := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	for _, p := range ps {
		if want[p.ID] {
			out[p.ID] = p.Name
		}
	}
	return out, nil
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
