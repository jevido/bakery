// Package servers is what the router and the boot code may use from the
// servers context. Nothing else in contexts/servers is for outside use.
package servers

import (
	"context"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
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
	})
	return service
}

// Start makes sure the Local server exists and validates it, retrying in
// the background while the database is unreachable.
func Start(ctx context.Context) {
	s := svc()
	go func() {
		for attempt := 1; ; attempt++ {
			local, err := s.EnsureLocal(ctx)
			if err == nil {
				_, err = s.Validate(ctx, local.ID)
			}
			if err == nil {
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
