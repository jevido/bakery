// Package deployments is what the router and the boot code may use from the
// deployments context: its routes and the Worker. Nothing else in
// contexts/deployments is for outside use.
package deployments

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	deploymentshttp "github.com/jevido/bakery/services/api/contexts/deployments/http"
	"github.com/jevido/bakery/services/api/contexts/deployments/infra"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/routing"
)

var (
	once    sync.Once
	service *app.Service
	runtime infra.Runtime
)

// applications translates projects' snapshot into this context's language.
func applications(ctx context.Context, id uint64) (app.Application, error) {
	s, err := projects.ApplicationForDeploy(ctx, id)
	if err != nil {
		return app.Application{}, err
	}
	return app.Application{
		ID: s.ID, Slug: s.Slug, GitURL: s.GitURL, GitBranch: s.GitBranch,
		DockerfilePath: s.DockerfilePath, Port: s.Port, Domain: s.Domain, Env: s.Env, DeployKey: s.DeployKey,
	}, nil
}

func svc() *app.Service {
	once.Do(func() {
		service = app.NewService(infra.Store{}, infra.Logs{}, applications)
		runtime = infra.Runtime{
			Podman:       podman.Default(),
			Network:      facades.Config().GetString("bakery.network"),
			StartTimeout: 30 * time.Second,
			Settle:       2 * time.Second,
		}
		projects.OnApplicationDeleted(func(ctx context.Context, applicationID uint64) {
			if err := runtime.RemoveAll(ctx, applicationID); err != nil {
				facades.Log().Errorf("deployments: removing containers of application %d: %v", applicationID, err)
			}
			if err := (infra.Store{}).DeleteForApplication(ctx, applicationID); err != nil {
				facades.Log().Errorf("deployments: deleting deployments of application %d: %v", applicationID, err)
			}
		})
	})
	return service
}

// Routes registers the deployments API, all behind identity.Auth.
func Routes(r route.Router) {
	c := deploymentshttp.NewController(svc(), func(err error) bool { return errors.Is(err, projects.ErrNotFound) })
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Post("/api/applications/{id}/deploy", c.Deploy)
		r.Get("/api/applications/{id}/deployments", c.List)
		r.Get("/api/deployments/{id}", c.Show)
	})
}

var shutdown = make(chan struct{})

// StreamRoutes registers the live log streams, behind identity.Auth but
// outside the request timeout.
func StreamRoutes(r route.Router) {
	isNotFound := func(err error) bool { return errors.Is(err, projects.ErrNotFound) || errors.Is(err, app.ErrNotFound) }
	c := deploymentshttp.NewStreamController(svc(), followContainer, isNotFound, shutdown)
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Get("/api/deployments/{id}/log", c.DeploymentLog)
		r.Get("/api/applications/{id}/logs", c.ContainerLogs)
	})
}

// followContainer follows the Application's running Container.
func followContainer(ctx context.Context, applicationID uint64, tail int, out func(stream, line string)) (bool, error) {
	name, found, err := runtime.Running(ctx, applicationID)
	if err != nil || !found {
		return false, err
	}
	return true, runtime.Podman.Logs(ctx, name, true, tail, func(stream, line string) {
		if stream == "stderr" {
			out("err", line)
		} else {
			out("out", line)
		}
	})
}

// StartWorker fails Deployments a previous process left half done, then
// runs the Worker until ctx ends.
func StartWorker(ctx context.Context) {
	workDir := filepath.Join(os.TempDir(), "bakery-builds")
	w := app.NewWorker(svc(), infra.Git{}, runtime, routing.SwitchRoute, workDir)
	w.Log = facades.Log().Errorf
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()
	go func() {
		for attempt := 1; ; attempt++ {
			if err := w.Recover(ctx); err == nil {
				break
			} else {
				facades.Log().Errorf("deployments: recovering (attempt %d): %v", attempt, err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
		w.Run(ctx)
	}()
}
