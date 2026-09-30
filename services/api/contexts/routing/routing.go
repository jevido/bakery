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
		var dashboard *domain.DashboardRoute
		if d := cfg.GetString("bakery.dashboard.domain"); d != "" {
			dashboard = &domain.DashboardRoute{
				Domain: d,
				API:    cfg.GetString("bakery.dashboard.api_upstream"),
				Web:    cfg.GetString("bakery.dashboard.web_upstream"),
			}
		}
		proxy := infra.NewProxy(podman.Default(), infra.ProxyConfig{
			Name:         "bakery-proxy",
			Image:        cfg.GetString("bakery.proxy.image"),
			Network:      cfg.GetString("bakery.network"),
			BindIP:       cfg.GetString("bakery.proxy.bind"),
			HTTPPort:     uint16(cfg.GetInt("bakery.proxy.http_port")),
			HTTPSPort:    uint16(cfg.GetInt("bakery.proxy.https_port")),
			AdminURL:     cfg.GetString("bakery.proxy.admin_url"),
			AdminPublish: cfg.GetString("bakery.proxy.admin_publish"),
			InternalTLS:  cfg.GetBool("bakery.proxy.internal_tls"),
			Dashboard:    dashboard,
			ACMECA:       cfg.GetString("bakery.acme.ca"),
			ACMEEmail:    cfg.GetString("bakery.acme.email"),
			ACMERoot:     cfg.GetString("bakery.acme.ca_root"),
			VolumePrefix: "bakery-proxy",
		})
		service = app.NewService(infra.Routes{}, proxy)
		projects.OnApplicationDomainsChanged(func(ctx context.Context, applicationID uint64, domains []string) {
			if err := service.ChangeDomains(ctx, applicationID, domains); err != nil {
				facades.Log().Errorf("routing: moving the route of application %d to %v: %v", applicationID, domains, err)
			}
		})
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

// SwitchRoute points an Application's Domains at a Container and port, and
// Applies. When it returns nil, Caddy is serving the new Container.
func SwitchRoute(ctx context.Context, applicationID uint64, domains []string, container string, port int) error {
	return svc().SwitchRoute(ctx, domain.Route{ApplicationID: applicationID, Domains: domains, Container: container, Port: port})
}
