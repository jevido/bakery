// Package servers is what the router and the boot code may use from the
// servers context. Nothing else in contexts/servers is for outside use.
package servers

import (
	"context"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
	servershttp "github.com/jevido/bakery/services/api/contexts/servers/http"
	"github.com/jevido/bakery/services/api/contexts/servers/infra"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		sock := facades.Config().GetString("bakery.podman_socket")
		if sock == "" {
			sock = podman.DefaultSocket()
		}
		service = app.NewService(infra.Store{}, infra.NewServerKey, infra.Connector{Local: podman.Default(), LocalSocket: sock})
		service.Retention = deployments.PruneImages
		service.Log = facades.Log().Errorf
	})
	return service
}

// Routes registers the servers API, all behind identity.Auth.
func Routes(r route.Router) {
	c := servershttp.NewController(svc())
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Get("/api/servers", c.List)
		r.Post("/api/servers", c.Create)
		r.Get("/api/servers/{id}", c.Show)
		r.Patch("/api/servers/{id}", c.Update)
		r.Delete("/api/servers/{id}", c.Delete)
		r.Delete("/api/servers/{id}/host-key", c.ForgetHostKey)
		r.Get("/api/servers/{id}/metrics", c.Metrics)
	})
}

// LongRoutes registers Validate and Clean up, which talk to a Server for
// longer than the request timeout allows (a Validation up to 30 s, a
// Cleanup minutes), behind identity.Auth.
func LongRoutes(r route.Router) {
	c := servershttp.NewController(svc())
	r.Middleware(identity.Auth).Group(func(r route.Router) {
		r.Post("/api/servers/{id}/validate", c.Validate)
		r.Post("/api/servers/{id}/cleanup", c.CleanUp)
	})
}

// Start makes sure the Local server exists and validates it, retrying in
// the background while the database is unreachable, then runs the daily
// Cleanup.
func Start(ctx context.Context) {
	s := svc()
	go func() {
		for attempt := 1; ; attempt++ {
			local, err := s.EnsureLocal(ctx)
			if err == nil {
				_, err = s.Validate(ctx, local.ID)
			}
			if err == nil {
				go daily(ctx, s)
				return
			}
			facades.Log().Errorf("servers: local server (attempt %d): %v", attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(min(time.Duration(attempt)*2*time.Second, 30*time.Second)):
			}
		}
	}()
}

// cleanupHour is the hour (server time) the daily Cleanup runs at.
const cleanupHour = 3

// daily cleans up every Reachable Server at cleanupHour each day until ctx
// ends.
func daily(ctx context.Context, s *app.Service) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), cleanupHour, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
		if err := s.CleanUpAll(ctx); err != nil && ctx.Err() == nil {
			facades.Log().Errorf("servers: daily cleanup: %v", err)
		}
	}
}
