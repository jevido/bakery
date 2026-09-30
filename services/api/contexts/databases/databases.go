// Package databases is what the router and the boot code may use from the
// databases context: its routes, the log stream and Recover. Nothing else in
// contexts/databases is for outside use.
package databases

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/databases/app"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
	databaseshttp "github.com/jevido/bakery/services/api/contexts/databases/http"
	"github.com/jevido/bakery/services/api/contexts/databases/infra"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/projects"
)

var (
	once    sync.Once
	service *app.Service
	runtime infra.Runtime
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

// publicHost is the host Public URLs name.
func publicHost() string {
	cfg := facades.Config()
	for _, h := range []string{cfg.GetString("bakery.databases.public_host"), cfg.GetString("bakery.dashboard.domain")} {
		if h != "" {
			return h
		}
	}
	return "localhost"
}

func svc() *app.Service {
	once.Do(func() {
		cfg := facades.Config()
		runtime = infra.Runtime{
			Podman:     podman.Default(),
			Network:    cfg.GetString("bakery.network"),
			PublicBind: cfg.GetString("bakery.databases.public_bind"),
		}
		service = app.NewService(infra.Store{}, runtime, environments, publicHost())
		service.Log = facades.Log().Errorf
		service.Files = infra.BackupFiles{Dir: cfg.GetString("bakery.backups.dir")}
		service.S3 = func(st domain.S3Storage) app.S3Client { return infra.S3{Storage: st} }
		projects.OnProjectDeleting(service.InUse)
	})
	return service
}

// Routes registers the databases API, all behind identity.Auth.
func Routes(r route.Router) {
	c := databaseshttp.NewController(svc())
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Post("/api/environments/{id}/databases", c.Create)
		r.Get("/api/projects/{id}/databases", c.ForProject)
		r.Get("/api/databases/{id}", c.Show)
		r.Patch("/api/databases/{id}", c.Update)
		r.Delete("/api/databases/{id}", c.Delete)
		r.Post("/api/databases/{id}/start", c.Start)
		r.Post("/api/databases/{id}/stop", c.Stop)
		r.Post("/api/databases/{id}/restart", c.Restart)
	})
}

// StreamRoutes registers the log stream, behind identity.Auth but outside
// the request timeout.
func StreamRoutes(r route.Router) {
	c := databaseshttp.NewStreamController(svc(), shutdown)
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Get("/api/databases/{id}/logs", c.Logs)
	})
}

// Recover starts, in the background, every Database that should run and has
// no Container, retrying while Podman or the database is unreachable, and
// then the scheduler of Backups.
func Recover(ctx context.Context) {
	s := svc()
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()

	go func() {
		for attempt := 1; ; attempt++ {
			err := runtime.Podman.EnsureNetwork(ctx, runtime.Network)
			if err == nil {
				err = s.Recover(ctx)
			}
			if err == nil {
				// Only now: Recover marks Backups left running as failed,
				// which must not hit one the scheduler just started.
				go schedule(ctx, s)
				return
			}
			facades.Log().Errorf("databases: recovering (attempt %d): %v", attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(min(time.Duration(attempt)*2*time.Second, 30*time.Second)):
			}
		}
	}()
}

// schedule runs the Backup scheduler at the start of every minute until
// ctx ends.
func schedule(ctx context.Context, s *app.Service) {
	for {
		now := time.Now()
		select {
		case <-ctx.Done():
			return
		case <-time.After(now.Truncate(time.Minute).Add(time.Minute).Sub(now)):
		}
		if err := s.Tick(ctx, time.Now()); err != nil && ctx.Err() == nil {
			facades.Log().Errorf("databases: backup scheduler: %v", err)
		}
	}
}
