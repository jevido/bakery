// Package infra runs the Proxy (Caddy) through Podman, configures it through
// Caddy's admin API, and stores Routes with the Goravel ORM.
package infra

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// ProxyConfig says how the Proxy container is run.
type ProxyConfig struct {
	Name      string // container name, bakery-proxy
	Image     string // docker.io/library/caddy:2
	Network   string // bakery
	BindIP    string // host address for :80 and :443; empty = all
	HTTPPort  uint16 // host port for :80
	HTTPSPort uint16 // host port for :443
	AdminURL  string // how the API reaches the admin API
	// Host address the admin API is published on, 127.0.0.1:4949 in dev;
	// empty means it is not published (a server, where the API reaches it
	// on the network).
	AdminPublish string
	InternalTLS  bool
	// Dashboard is the Dashboard Route; nil when no dashboard domain is
	// configured (development).
	Dashboard *domain.DashboardRoute
	ACMECA    string
	ACMEEmail string
	// ACMERoot is a PEM file on the API's side, copied into the Proxy for
	// Caddy to trust the ACME CA's HTTPS certificate; empty for none.
	ACMERoot string
	// Volume prefix for Caddy's /data (certificates, the internal CA) and
	// /config (the autosaved last config).
	VolumePrefix string
}

type Proxy struct {
	podman *podman.Client
	cfg    ProxyConfig
	caddy  *Caddy
}

func NewProxy(p *podman.Client, cfg ProxyConfig) *Proxy {
	return &Proxy{podman: p, cfg: cfg, caddy: &Caddy{AdminURL: cfg.AdminURL}}
}

// Ensure makes sure the network exists and the Proxy container exists and
// runs, then waits for its admin API.
func (p *Proxy) Ensure(ctx context.Context) error {
	if err := p.podman.EnsureNetwork(ctx, p.cfg.Network); err != nil {
		return fmt.Errorf("network %s: %w", p.cfg.Network, err)
	}
	info, err := p.podman.InspectContainer(ctx, p.cfg.Name)
	switch {
	case podman.IsNotFound(err):
		if err := p.create(ctx); err != nil {
			return err
		}
	case err != nil:
		return err
	case !info.State.Running:
		if err := p.podman.StartContainer(ctx, p.cfg.Name); err != nil {
			return fmt.Errorf("starting %s: %w", p.cfg.Name, err)
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := p.caddy.WaitReady(waitCtx); err != nil {
		return err
	}
	return p.copyACMERoot(ctx)
}

// acmeRootInProxy is where the extra ACME root lands, on the /data volume.
const acmeRootInProxy = "/data/bakery/acme-root.pem"

// copyACMERoot puts the configured ACME root into the Proxy, every time: a
// recreated Proxy has a fresh volume, and the file may have changed.
func (p *Proxy) copyACMERoot(ctx context.Context) error {
	if p.cfg.ACMERoot == "" {
		return nil
	}
	pem, err := os.ReadFile(p.cfg.ACMERoot)
	if err != nil {
		return fmt.Errorf("ACME root: %w", err)
	}
	if err := p.podman.CopyInto(ctx, p.cfg.Name, "/data", map[string][]byte{"bakery/acme-root.pem": pem}); err != nil {
		return fmt.Errorf("copying the ACME root into %s: %w", p.cfg.Name, err)
	}
	return nil
}

func (p *Proxy) create(ctx context.Context) error {
	if ok, err := p.podman.ImageExists(ctx, p.cfg.Image); err != nil {
		return err
	} else if !ok {
		if err := p.podman.PullImage(ctx, p.cfg.Image, nil); err != nil {
			return fmt.Errorf("pulling %s: %w", p.cfg.Image, err)
		}
	}
	ports := []podman.PortMapping{
		{HostIP: p.cfg.BindIP, HostPort: p.cfg.HTTPPort, ContainerPort: 80, Protocol: "tcp"},
		{HostIP: p.cfg.BindIP, HostPort: p.cfg.HTTPSPort, ContainerPort: 443, Protocol: "tcp"},
	}
	if p.cfg.AdminPublish != "" {
		adminHost, adminPort, err := net.SplitHostPort(p.cfg.AdminPublish)
		if err != nil {
			return fmt.Errorf("proxy admin address %q: %w", p.cfg.AdminPublish, err)
		}
		adminPortNum, err := strconv.ParseUint(adminPort, 10, 16)
		if err != nil {
			return fmt.Errorf("proxy admin port %q: %w", adminPort, err)
		}
		// The admin API has no authentication: never publish it beyond
		// the host address configured (127.0.0.1).
		ports = append(ports, podman.PortMapping{HostIP: adminHost, HostPort: uint16(adminPortNum), ContainerPort: 2019, Protocol: "tcp"})
	}
	id, err := p.podman.CreateContainer(ctx, podman.ContainerSpec{
		Name:  p.cfg.Name,
		Image: p.cfg.Image,
		// --resume loads the config Caddy autosaved in /config, so after a
		// host reboot the Proxy serves the last Routes before Bakery is up.
		Command: []string{"caddy", "run", "--resume"},
		// Only used until the first Apply, which sets the same address.
		Env:          map[string]string{"CADDY_ADMIN": CaddyAdminListen},
		Labels:       map[string]string{"bakery.managed": "true", "bakery.role": "proxy"},
		Networks:     podman.OnNetwork(p.cfg.Network),
		PortMappings: ports,
		Volumes: []podman.NamedVolume{
			{Name: p.cfg.VolumePrefix + "-data", Dest: "/data"},
			{Name: p.cfg.VolumePrefix + "-config", Dest: "/config"},
		},
		RestartPolicy: "always",
	})
	if err != nil {
		return fmt.Errorf("creating %s: %w", p.cfg.Name, err)
	}
	if err := p.podman.StartContainer(ctx, id); err != nil {
		return fmt.Errorf("starting %s: %w", p.cfg.Name, err)
	}
	return nil
}

// Apply renders the Routes and loads them.
func (p *Proxy) Apply(ctx context.Context, routes []domain.Route) error {
	opts := RenderOptions{
		InternalTLS: p.cfg.InternalTLS,
		Dashboard:   p.cfg.Dashboard,
		ACME:        ACME{CA: p.cfg.ACMECA, Email: p.cfg.ACMEEmail},
		HTTPSPort:   int(p.cfg.HTTPSPort),
	}
	if p.cfg.ACMERoot != "" {
		opts.ACME.TrustedRootsFile = acmeRootInProxy
	}
	config, err := Render(routes, opts)
	if err != nil {
		return err
	}
	return p.caddy.Load(ctx, config)
}
