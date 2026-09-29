// Package routing is what other contexts and the boot code may use from the
// routing context: EnsureProxy at start, SwitchRoute for deployments.
// Nothing else in contexts/routing is for outside use.
package routing

import (
	"context"
	"sync"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/routing/app"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
	"github.com/jevido/bakery/services/api/contexts/routing/infra"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		cfg := facades.Config()
		proxy := infra.NewProxy(podman.Default(), infra.ProxyConfig{
			Name:         "bakery-proxy",
			Image:        cfg.GetString("bakery.proxy.image"),
			Network:      cfg.GetString("bakery.network"),
			BindIP:       cfg.GetString("bakery.proxy.bind"),
			HTTPPort:     uint16(cfg.GetInt("bakery.proxy.http_port")),
			HTTPSPort:    uint16(cfg.GetInt("bakery.proxy.https_port")),
			AdminAddr:    cfg.GetString("bakery.proxy.admin"),
			InternalTLS:  cfg.GetBool("bakery.proxy.internal_tls"),
			VolumePrefix: "bakery-proxy",
		})
		service = app.NewService(infra.Routes{}, proxy)
		projects.OnApplicationDeleted(func(ctx context.Context, applicationID uint64) {
			if err := service.DropRoute(ctx, applicationID); err != nil {
				facades.Log().Errorf("routing: dropping route of application %d: %v", applicationID, err)
			}
		})
	})
	return service
}

// Init wires routing's event handlers. Call once at start.
func Init() { svc() }

// EnsureProxy makes sure the network and the Proxy exist and run, and loads
// every Route into it.
func EnsureProxy(ctx context.Context) error {
	return svc().EnsureProxy(ctx)
}

// SwitchRoute points an Application's Domain at a Container and port, and
// Applies. When it returns nil, Caddy is serving the new Container.
func SwitchRoute(ctx context.Context, applicationID uint64, domainName, container string, port int) error {
	return svc().SwitchRoute(ctx, domain.Route{ApplicationID: applicationID, Domain: domainName, Container: container, Port: port})
}
