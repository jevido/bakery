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
		service = app.NewService(infra.Store{}, facades.Config().GetString("bakery.domain_suffix", "localhost"))
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
		r.Get("/api/applications/{id}/env", c.ShowEnv)
		r.Put("/api/applications/{id}/env", c.ReplaceEnv)
	})
}

// ApplicationSnapshot is an Application as deployments needs it, taken once
// at the start of a Deployment. Env var values are decrypted.
type ApplicationSnapshot struct {
	ID             uint64
	Slug           string
	GitURL         string
	GitBranch      string
	DockerfilePath string
	Port           int
	Domain         string
	Env            map[string]string
}

// ApplicationForDeploy returns the snapshot, or ErrNotFound.
func ApplicationForDeploy(ctx context.Context, id uint64) (ApplicationSnapshot, error) {
	a, err := svc().Application(ctx, id)
	if err != nil {
		return ApplicationSnapshot{}, err
	}
	vars, err := svc().EnvVars(ctx, id)
	if err != nil {
		return ApplicationSnapshot{}, err
	}
	env := make(map[string]string, len(vars))
	for _, v := range vars {
		env[v.Name] = v.Value
	}
	return ApplicationSnapshot{
		ID: a.ID, Slug: a.Slug, GitURL: a.GitURL, GitBranch: a.GitBranch,
		DockerfilePath: a.DockerfilePath, Port: a.Port, Domain: a.Domain, Env: env,
	}, nil
}

// OnApplicationDeleted registers a handler for the ApplicationDeleted event.
func OnApplicationDeleted(f func(ctx context.Context, applicationID uint64)) {
	svc().OnApplicationDeleted(f)
}
