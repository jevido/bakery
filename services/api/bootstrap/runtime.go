package bootstrap

import (
	"context"
	"time"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/databases"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/routing"
)

// StartRuntime starts what runs next to the HTTP server: the Proxy, the
// deployment Worker and the Databases that should run. It is
// not called for artisan commands. Failures are retried in the background
// so a Podman or database hiccup at start does not keep the API down.
func StartRuntime(ctx context.Context) {
	routing.Init()
	deployments.StartWorker(ctx)
	databases.Recover(ctx)
	go func() {
		for attempt := 1; ; attempt++ {
			err := routing.EnsureProxy(ctx)
			if err == nil {
				facades.Log().Info("proxy: bakery-proxy is running and configured")
				return
			}
			facades.Log().Errorf("proxy: attempt %d: %v", attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(min(time.Duration(attempt)*2*time.Second, 30*time.Second)):
			}
		}
	}()
}
