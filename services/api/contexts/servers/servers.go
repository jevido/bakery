// Package servers is what the router and the boot code may use from the
// servers context. Nothing else in contexts/servers is for outside use.
package servers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
	servershttp "github.com/jevido/bakery/services/api/contexts/servers/http"
	"github.com/jevido/bakery/services/api/contexts/servers/infra"
)

var (
	once    sync.Once
	service *app.Service
	pool    *infra.Pool
)

func svc() *app.Service {
	once.Do(func() {
		sock := facades.Config().GetString("bakery.podman_socket")
		if sock == "" {
			sock = podman.DefaultSocket()
		}
		service = app.NewService(infra.Store{}, infra.NewPrivateKey, infra.Connector{Local: podman.Default(), LocalSocket: sock})
		pool = &infra.Pool{Local: podman.Default()}
		service.Forget = pool.Forget
		service.Log = facades.Log().Errorf
		service.OnHealthChanged = publishHealthChanged
	})
	return service
}

var (
	// ErrNotFound is a Server that does not exist.
	ErrNotFound = app.ErrNotFound
	// ErrHostKeyChanged is a Remote server presenting another host key than
	// the pinned one; nothing is sent to it until the Owner forgets it.
	ErrHostKeyChanged = errors.New("the server's host key changed")
	// ErrNotValidated is a Remote server that was never validated, so its
	// host key is not known yet.
	ErrNotValidated = app.ErrNotValidated
)

// Connection reaches one Server for another context. It is pooled: do not
// close anything it hands out, except the net.Conns DialUnix returns.
type Connection struct {
	// ServerID is 0 for the Local server.
	ServerID uint64
	Local    bool
	Name     string
	// Host is the address the Server is reached on (localhost for the
	// Local server), where its Domains' DNS must point.
	Host     string
	Podman   *podman.Client
	DialUnix func(ctx context.Context, path string) (net.Conn, error)
}

// Connect returns a Server connection; serverID 0 is the Local server. A
// Remote server must have been validated (its host key pinned), and one
// that presents another host key is refused with ErrHostKeyChanged.
func Connect(ctx context.Context, serverID uint64) (Connection, error) {
	srv, err := svc().Reach(ctx, serverID)
	if err != nil {
		if errors.Is(err, app.ErrNotValidated) {
			return Connection{}, fmt.Errorf("server %s: %w", srv.Name, err)
		}
		return Connection{}, err
	}
	r, err := pool.Reach(ctx, srv)
	var changed *app.HostKeyChangedError
	if errors.As(err, &changed) {
		return Connection{}, fmt.Errorf("server %s: %w: %s", srv.Name, ErrHostKeyChanged, changed.Detail)
	}
	if err != nil {
		return Connection{}, fmt.Errorf("server %s (%s:%d) is not reachable: %w", srv.Name, srv.Host, srv.Port, err)
	}
	return Connection{
		ServerID: srv.RefID(), Local: srv.RefID() == 0, Name: srv.Name, Host: srv.Host,
		Podman: r.Podman, DialUnix: r.DialUnix,
	}, nil
}

// LocalID is the Local server's id, for showing it; every other context
// names the Local server 0.
func LocalID(ctx context.Context) (uint64, error) {
	srv, err := svc().EnsureLocal(ctx)
	return srv.ID, err
}

// Exists reports whether the Server exists; 0 (the Local server) always
// does.
func Exists(ctx context.Context, id uint64) (bool, error) {
	if id == 0 {
		return true, nil
	}
	_, err := svc().Get(ctx, id)
	if errors.Is(err, app.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// OnServerDeleting registers a check asked before a Server is deleted: while
// it reports true the Server stays. The Local server is never deleted.
func OnServerDeleting(inUse func(ctx context.Context, serverID uint64) (bool, error)) {
	svc().OnDeleting(inUse)
}

// OnCleanup registers the Image retention run during every Cleanup, with
// the Server's id (0 for the Local server); it returns the bytes freed.
func OnCleanup(retention func(ctx context.Context, serverID uint64) (int64, error)) {
	svc().Retention = retention
}

// Routes registers the servers API behind guilds.Auth. Every Member may
// list the Servers (to pick a Target server) and see their usage; only
// admins change them.
func Routes(r route.Router) {
	c := servershttp.NewController(svc())
	r.Middleware(guilds.Auth).Group(func(r route.Router) {
		r.Get("/api/servers", c.List)
		r.Get("/api/servers/{id}", c.Show)
		r.Get("/api/servers/{id}/metrics", c.Metrics)
		r.Get("/api/servers/{id}/details", c.Details)
	})
	r.Middleware(guilds.Auth, guilds.Admin).Group(func(r route.Router) {
		r.Post("/api/servers", c.Create)
		r.Patch("/api/servers/{id}", c.Update)
		r.Delete("/api/servers/{id}", c.Delete)
		r.Delete("/api/servers/{id}/host-key", c.ForgetHostKey)
	})
}

// LongRoutes registers Validate and Clean up, which talk to a Server for
// longer than the request timeout allows (a Validation up to 30 s, a
// Cleanup minutes), behind guilds.Auth and guilds.Admin.
func LongRoutes(r route.Router) {
	c := servershttp.NewController(svc())
	r.Middleware(guilds.Auth, guilds.Admin).Group(func(r route.Router) {
		r.Post("/api/servers/{id}/validate", c.Validate)
		r.Post("/api/servers/{id}/cleanup", c.CleanUp)
	})
}

// Start makes sure the Local server exists and validates it, retrying in
// the background while the database is unreachable, then runs the daily
// Cleanup and the Server probe.
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
				go probe(ctx, s)
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

// probeInterval is how often every Server is probed.
func probeInterval() time.Duration {
	d, err := time.ParseDuration(facades.Config().GetString("bakery.servers.probe_interval"))
	if err != nil || d < time.Second {
		return 5 * time.Minute
	}
	return d
}

// probe runs the Server probe every probeInterval until ctx ends.
func probe(ctx context.Context, s *app.Service) {
	every := probeInterval()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
		if err := s.ProbeAll(ctx); err != nil && ctx.Err() == nil {
			facades.Log().Errorf("servers: probe: %v", err)
		}
	}
}

// ServerHealthChanged is what a Server probe found changed about a Server.
type ServerHealthChanged struct {
	ServerID   uint64
	ServerName string
	// Change is "unreachable", "reachable" or "server_disk_usage".
	Change string
	// Reason is why an unreachable Server could not be reached.
	Reason    string
	DiskUsed  int64
	DiskTotal int64
}

var (
	healthMu        sync.Mutex
	onHealthChanged []func(ctx context.Context, e ServerHealthChanged)
)

// OnServerHealthChanged registers f to hear of every ServerHealthChanged.
// It runs in its own goroutine, so it can neither hold up nor break the
// probe.
func OnServerHealthChanged(f func(ctx context.Context, e ServerHealthChanged)) {
	healthMu.Lock()
	defer healthMu.Unlock()
	onHealthChanged = append(onHealthChanged, f)
}

func publishHealthChanged(_ context.Context, h app.HealthChanged) {
	e := ServerHealthChanged{
		ServerID: h.Server.ID, ServerName: h.Server.Name, Change: string(h.Change),
		Reason: h.Reason, DiskUsed: h.DiskUsed, DiskTotal: h.DiskTotal,
	}
	healthMu.Lock()
	subscribers := slices.Clone(onHealthChanged)
	healthMu.Unlock()
	for _, f := range subscribers {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					facades.Log().Errorf("servers: a ServerHealthChanged subscriber panicked: %v", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f(ctx, e)
		}()
	}
}
