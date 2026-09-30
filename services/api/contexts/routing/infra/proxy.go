// Package infra runs the Proxy (Caddy) through Podman, configures it through
// Caddy's admin API, and stores Routes with the Goravel ORM.
package infra

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/routing/app"
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

// Remote Proxy admin socket: a volume mounted into the Proxy, so the socket
// Caddy creates there is a file on the Server that SSH can open.
const (
	adminVolumeSuffix = "-admin"
	adminSocketDir    = "/run/bakery-admin"
	adminSocketName   = "caddy.sock"
)

type Proxy struct {
	podman *podman.Client
	cfg    ProxyConfig
	caddy  *Caddy
	// dial opens a unix socket on the Proxy's Server; set for a Remote
	// Proxy, whose admin API is a socket rather than a URL.
	dial func(ctx context.Context, path string) (net.Conn, error)
	// adminMu guards connecting caddy to a Remote Proxy's socket.
	adminMu sync.Mutex
}

func NewProxy(p *podman.Client, cfg ProxyConfig) *Proxy {
	return &Proxy{podman: p, cfg: cfg, caddy: &Caddy{AdminURL: cfg.AdminURL}}
}

// NewRemoteProxy is the Proxy of a Remote server: p is that Server's Podman
// and dial opens unix sockets on it. It listens on 80/443 of every address
// of the Server and never serves the Dashboard Route; its admin API is a
// unix socket in a volume, published on no port.
func NewRemoteProxy(p *podman.Client, dial func(ctx context.Context, path string) (net.Conn, error), cfg ProxyConfig) *Proxy {
	cfg.BindIP, cfg.HTTPPort, cfg.HTTPSPort = "", 80, 443
	cfg.AdminURL, cfg.AdminPublish, cfg.Dashboard = "", "", nil
	return &Proxy{podman: p, cfg: cfg, dial: dial}
}

func (p *Proxy) remote() bool { return p.dial != nil }

func (p *Proxy) adminVolume() string { return p.cfg.VolumePrefix + adminVolumeSuffix }

// adminListen is the admin API address inside the Proxy.
func (p *Proxy) adminListen() string {
	if p.remote() {
		return "unix/" + adminSocketDir + "/" + adminSocketName
	}
	return CaddyAdminListen
}

// connectAdmin points a Remote Proxy's Caddy client at the socket in its
// admin volume on the Server.
func (p *Proxy) connectAdmin(ctx context.Context) error {
	if !p.remote() {
		return nil
	}
	p.adminMu.Lock()
	defer p.adminMu.Unlock()
	if p.caddy != nil {
		return nil
	}
	dir, err := p.podman.VolumeMountpoint(ctx, p.adminVolume())
	if err != nil {
		return fmt.Errorf("volume %s: %w", p.adminVolume(), err)
	}
	sock := dir + "/" + adminSocketName
	p.caddy = newSocketCaddy(func(ctx context.Context) (net.Conn, error) { return p.dial(ctx, sock) })
	return nil
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
	if err := p.connectAdmin(ctx); err != nil {
		return err
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
	labels := map[string]string{"bakery.managed": "true", "bakery.role": "proxy"}
	volumes := []podman.NamedVolume{
		{Name: p.cfg.VolumePrefix + "-data", Dest: "/data"},
		{Name: p.cfg.VolumePrefix + "-config", Dest: "/config"},
	}
	if p.remote() {
		// Created with Bakery's labels, so its mountpoint can be read
		// before the container exists.
		if err := p.podman.CreateVolume(ctx, p.adminVolume(), labels); err != nil {
			return fmt.Errorf("volume %s: %w", p.adminVolume(), err)
		}
		volumes = append(volumes, podman.NamedVolume{Name: p.adminVolume(), Dest: adminSocketDir})
	}
	id, err := p.podman.CreateContainer(ctx, podman.ContainerSpec{
		Name:  p.cfg.Name,
		Image: p.cfg.Image,
		// --resume loads the config Caddy autosaved in /config, so after a
		// host reboot the Proxy serves the last Routes before Bakery is up.
		Command: []string{"caddy", "run", "--resume"},
		// Only used until the first Apply, which sets the same address.
		Env:           map[string]string{"CADDY_ADMIN": p.adminListen()},
		Labels:        labels,
		Networks:      podman.OnNetwork(p.cfg.Network),
		PortMappings:  ports,
		Volumes:       volumes,
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
		AdminListen: p.adminListen(),
	}
	if p.cfg.ACMERoot != "" {
		opts.ACME.TrustedRootsFile = acmeRootInProxy
	}
	config, err := Render(routes, opts)
	if err != nil {
		return err
	}
	if err := p.connectAdmin(ctx); err != nil {
		return err
	}
	return p.caddy.Load(ctx, config)
}

// Dial opens a unix socket on a Server.
type Dial func(ctx context.Context, path string) (net.Conn, error)

// Proxies hands out the Proxy of each Server: the Local one as configured,
// Remote ones built on that Server's connection with the same config.
type Proxies struct {
	Local  *Proxy
	Config ProxyConfig
	// Connect returns the Podman client and socket dialer of a Remote
	// server (servers' pooled Server connection).
	Connect func(ctx context.Context, serverID uint64) (*podman.Client, Dial, error)

	mu     sync.Mutex
	remote map[uint64]*Proxy
}

// For returns the Server's Proxy. A Remote Proxy is rebuilt whenever the
// Server's connection was redialled, so it never talks through a dead one.
func (p *Proxies) For(ctx context.Context, serverID uint64) (app.Proxy, error) {
	if serverID == 0 {
		return p.Local, nil
	}
	client, dial, err := p.Connect(ctx, serverID)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cached, ok := p.remote[serverID]; ok && cached.podman == client {
		return cached, nil
	}
	if p.remote == nil {
		p.remote = map[uint64]*Proxy{}
	}
	proxy := NewRemoteProxy(client, dial, p.Config)
	p.remote[serverID] = proxy
	return proxy, nil
}
