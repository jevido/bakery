// Package services is what the router and the boot code may use from the
// services context: its routes, the log stream and Recover. Nothing else in
// contexts/services is for outside use.
package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/routing"
	"github.com/jevido/bakery/services/api/contexts/servers"
	"github.com/jevido/bakery/services/api/contexts/services/app"
	serviceshttp "github.com/jevido/bakery/services/api/contexts/services/http"
	"github.com/jevido/bakery/services/api/contexts/services/infra"
)

var (
	once    sync.Once
	service *app.Service
	// shutdown closes when the context Recover got ends, ending log
	// streams so a stopping API does not wait on open tabs.
	shutdown = make(chan struct{})
)

// environments translates projects' Environment into this context's.
func environments(ctx context.Context, id uint64) (app.Environment, error) {
	e, err := projects.Environment(ctx, id)
	if errors.Is(err, projects.ErrNotFound) {
		return app.Environment{}, app.ErrNotFound
	}
	return app.Environment{ID: e.ID, ProjectID: e.ProjectID}, err
}

// routes translates this context's Routes into routing's Service routes.
type routes struct{}

func (routes) Set(ctx context.Context, serviceID uint64, rs []app.Route) error {
	out := make([]routing.ServiceRoute, len(rs))
	for i, r := range rs {
		out[i] = routing.ServiceRoute{Component: r.Component, Domains: r.Domains, Container: r.Container, Port: r.Port}
	}
	return routing.SetServiceRoutes(ctx, serviceID, out)
}

func (routes) Drop(ctx context.Context, serviceID uint64) error {
	return routing.DropServiceRoutes(ctx, serviceID)
}

func svc() *app.Service {
	once.Do(func() {
		cfg := facades.Config()
		runtime := infra.Runtime{Podman: podman.Default(), Network: cfg.GetString("bakery.network")}
		service = app.NewService(infra.Store{}, runtime, routes{}, environments, projects.DomainInUse,
			cfg.GetString("bakery.domain_suffix"), infra.Generate)
		service.Log = facades.Log().Errorf
		catalog, err := infra.Templates()
		if err != nil {
			// The catalog is embedded and tested; a broken one is a build
			// mistake, not something to run with.
			panic(err)
		}
		service.SetTemplates(catalog)
		projects.OnProjectDeleting(service.InUse)
		projects.OnEnvironmentDeleting(service.InUseInEnvironment)
		projects.OnDomainCheck(service.DomainInUse)
		servers.OnServerResources("service", onServer)
		servers.OnContainerOwner("service", func(ctx context.Context, id, guildID uint64) (bool, error) {
			_, g, found, err := serviceProject(ctx, id)
			return found && g == guildID, err
		})
	})
	return service
}

var (
	environmentInProject = guilds.InProject("environment", projects.ProjectOf("environment"))
	projectInProject     = guilds.InProject("project", projects.ProjectOf("project"))
	// serviceInProject answers 404 for an {id} Service outside the Current
	// guild or in a Project the request may not view.
	serviceInProject = guilds.InProject("service", serviceProject)
)

// serviceProject finds the Project of the Service's Environment.
func serviceProject(ctx context.Context, id uint64) (uint64, uint64, bool, error) {
	envID, err := svc().EnvironmentOf(ctx, id)
	if errors.Is(err, app.ErrNotFound) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return projects.ProjectOf("environment")(ctx, envID)
}

// Routes registers the services API, all behind guilds.Auth, every route
// keyed by an Environment, Project or Service answering 404 outside the
// Current guild.
func Routes(r route.Router) {
	c := serviceshttp.NewController(svc())
	r.Middleware(guilds.Auth).Get("/api/service-templates", c.Templates)
	r.Middleware(guilds.Auth, environmentInProject, guilds.Can("manage_applications")).Post("/api/environments/{id}/services", c.Create)
	r.Middleware(guilds.Auth, projectInProject).Get("/api/projects/{id}/services", c.ForProject)
	r.Middleware(guilds.Auth, serviceInProject).Get("/api/services/{id}", c.Show)
	r.Middleware(guilds.Auth, serviceInProject, guilds.Can("manage_applications")).Group(func(r route.Router) {
		r.Patch("/api/services/{id}", c.Update)
		r.Delete("/api/services/{id}", c.Delete)
	})
	// Coolify's deploy actions: an API token needs deploy for them.
	r.Middleware(guilds.Deploy, serviceInProject, guilds.Can("deploy")).Group(func(r route.Router) {
		r.Post("/api/services/{id}/start", c.Start)
		r.Post("/api/services/{id}/stop", c.Stop)
		r.Post("/api/services/{id}/restart", c.Restart)
		r.Post("/api/services/{id}/redeploy", c.Redeploy)
	})
}

// StreamRoutes registers the Component log stream, behind guilds.Auth
// (404 outside the Current guild) but outside the request timeout.
func StreamRoutes(r route.Router) {
	c := serviceshttp.NewStreamController(svc(), shutdown)
	r.Middleware(guilds.Auth, serviceInProject).Get("/api/services/{id}/components/{component}/logs", c.Logs)
}

// Recover brings Up, in the background, every Service that should run and
// has a Component without a Container, retrying while Podman or the
// database is unreachable.
func Recover(ctx context.Context) {
	s := svc()
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()
	go func() {
		for attempt := 1; ; attempt++ {
			err := s.Recover(ctx)
			if err == nil {
				return
			}
			facades.Log().Errorf("services: recovering (attempt %d): %v", attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(min(time.Duration(attempt)*2*time.Second, 30*time.Second)):
			}
		}
	}()
}

// onServer lists the Services on a Server for its Resources list: they run on
// the Local server only.
func onServer(ctx context.Context, serverID uint64, in []servers.ResourceProject) ([]servers.ServerResource, error) {
	if serverID != 0 {
		return nil, nil
	}
	var out []servers.ServerResource
	for _, p := range in {
		list, err := service.ForProject(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		for _, v := range list {
			out = append(out, servers.ServerResource{ID: v.ID, Name: v.Name, EnvironmentID: v.EnvironmentID, Status: string(v.Status)})
		}
	}
	return out, nil
}
