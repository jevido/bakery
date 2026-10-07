// Package routing is what other contexts and the boot code may use from the
// routing context: EnsureProxy at start, SwitchRoute for deployments,
// SetServiceRoutes and DropServiceRoutes for services, and its routes (the
// Route settings API).
// Nothing else in contexts/routing is for outside use.
package routing

import (
	"context"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"

	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/routing/app"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
	routinghttp "github.com/jevido/bakery/services/api/contexts/routing/http"
	"github.com/jevido/bakery/services/api/contexts/routing/infra"
	"github.com/jevido/bakery/services/api/contexts/servers"
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
		cfgProxy := infra.ProxyConfig{
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
		}
		proxies := &infra.Proxies{
			Local:  infra.NewProxy(podman.Default(), cfgProxy),
			Config: cfgProxy,
			Connect: func(ctx context.Context, serverID uint64) (*podman.Client, infra.Dial, error) {
				conn, err := servers.Connect(ctx, serverID)
				if err != nil {
					return nil, nil, err
				}
				return conn.Podman, conn.DialUnix, nil
			},
		}
		service = app.NewService(infra.Routes{}, infra.ServiceRoutes{}, infra.PreviewRoutes{}, infra.Settings{}, proxies)
		projects.OnApplicationDomainsChanged(func(ctx context.Context, applicationID uint64, domains []string) {
			if err := service.ChangeDomains(ctx, applicationID, domains); err != nil {
				facades.Log().Errorf("routing: moving the route of application %d to %v: %v", applicationID, domains, err)
			}
		})
		projects.OnApplicationDeleted(func(ctx context.Context, e projects.ApplicationDeleted) {
			if err := service.DropRoute(ctx, e.ApplicationID); err != nil {
				facades.Log().Errorf("routing: dropping route of application %d: %v", e.ApplicationID, err)
			}
		})
	})
	return service
}

// Routes registers the Route settings API, behind guilds.Auth and
// guilds.InProject.
func Routes(r route.Router) {
	c := routinghttp.NewController(svc(), func(ctx contractshttp.Context, id uint64) (bool, error) {
		return projects.ApplicationInGuild(ctx.Context(), id, guilds.Current(ctx))
	})
	application := guilds.InProject("application", projects.ProjectOf("application"))
	r.Middleware(guilds.Auth, application).Get("/api/applications/{id}/routing", c.ShowSettings)
	r.Middleware(guilds.Auth, application, guilds.Can("manage_applications")).Put("/api/applications/{id}/routing", c.ReplaceSettings)
}

// Init wires routing's event handlers. Call once at start.
func Init() { svc() }

// EnsureProxy makes sure the network and the Local server's Proxy exist and
// run, and loads every Route there into it. Then, in the background, it
// does the same for every Remote server with Routes, retrying one that
// cannot be reached without holding up the others or the API.
func EnsureProxy(ctx context.Context) error {
	if err := svc().EnsureProxy(ctx); err != nil {
		return err
	}
	remote, err := svc().RemoteServers(ctx)
	if err != nil {
		return err
	}
	for _, id := range remote {
		go ensureRemote(ctx, id)
	}
	return nil
}

func ensureRemote(ctx context.Context, serverID uint64) {
	for attempt := 1; ; attempt++ {
		err := svc().EnsureRemoteProxy(ctx, serverID)
		if err == nil {
			facades.Log().Infof("proxy: the proxy of server %d is running and configured", serverID)
			return
		}
		facades.Log().Errorf("proxy: server %d, attempt %d: %v", serverID, attempt, err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(min(time.Duration(attempt)*5*time.Second, 5*time.Minute)):
		}
	}
}

// SwitchRoute points an Application's Domains at a Container and port on
// its Target server (0 the Local server), and Applies that Server's Proxy,
// creating it on the Server's first Route. When it returns nil, Caddy there
// is serving the new Container.
func SwitchRoute(ctx context.Context, serverID, applicationID uint64, domains []string, container string, port int) error {
	return svc().SwitchRoute(ctx, domain.Route{ApplicationID: applicationID, ServerID: serverID, Domains: domains, Container: container, Port: port})
}

// StopRoute stops serving a stopped Application's Domains; its Route
// settings and Preview routes stay. One without a Route is fine.
func StopRoute(ctx context.Context, applicationID uint64) error {
	return svc().StopRoute(ctx, applicationID)
}

// SwitchPreviewRoute points a Preview's Domains at a Container and port on
// the Application's Server and Applies that Server's Proxy. When it
// returns nil, Caddy there is serving the new Container.
func SwitchPreviewRoute(ctx context.Context, serverID, applicationID uint64, preview int, domains []string, container string, port int) error {
	return svc().SwitchPreviewRoute(ctx, domain.PreviewRoute{
		ApplicationID: applicationID, Preview: preview, ServerID: serverID, Domains: domains, Container: container, Port: port,
	})
}

// DropPreviewRoute stops serving a Preview; one that has no Preview route
// is fine.
func DropPreviewRoute(ctx context.Context, applicationID uint64, preview int) error {
	return svc().DropPreviewRoute(ctx, applicationID, preview)
}

// ServiceRoute is a Public Component's Domains, primary first, served from
// its Container and port.
type ServiceRoute struct {
	Component string
	Domains   []string
	Container string
	Port      int
}

// SetServiceRoutes makes routes the Service's whole set of Service routes
// and Applies. When it returns nil, Caddy serves them.
func SetServiceRoutes(ctx context.Context, serviceID uint64, routes []ServiceRoute) error {
	out := make([]domain.ServiceRoute, len(routes))
	for i, r := range routes {
		out[i] = domain.ServiceRoute{ServiceID: serviceID, Component: r.Component, Domains: r.Domains, Container: r.Container, Port: r.Port}
	}
	return svc().SetServiceRoutes(ctx, serviceID, out)
}

// DropServiceRoutes removes every Service route of the Service and Applies.
func DropServiceRoutes(ctx context.Context, serviceID uint64) error {
	return svc().DropServiceRoutes(ctx, serviceID)
}
