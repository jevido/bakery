//go:build podman

package infra

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// Runs against the Remote server stand-in (task remote:up):
// go test -tags podman ./contexts/servers/infra/

const standIn = "bakery-dev-remote-1"

// oneServer is a Store holding the Servers the test adds.
type oneServer struct{ servers map[uint64]domain.Server }

func (o *oneServer) Create(_ context.Context, s domain.Server) (domain.Server, error) {
	s.ID = uint64(len(o.servers) + 1)
	o.servers[s.ID] = s
	return s, nil
}
func (o *oneServer) Get(_ context.Context, id uint64) (domain.Server, bool, error) {
	s, ok := o.servers[id]
	return s, ok, nil
}
func (o *oneServer) List(context.Context) ([]domain.Server, error) { return nil, nil }
func (o *oneServer) Save(_ context.Context, s domain.Server) error {
	o.servers[s.ID] = s
	return nil
}
func (o *oneServer) Delete(context.Context, uint64) error { return nil }
func (o *oneServer) NameTaken(context.Context, string, uint64) (bool, error) {
	return false, nil
}
func (o *oneServer) AddressTaken(context.Context, string, int, string, uint64) (bool, error) {
	return false, nil
}
func (o *oneServer) Local(context.Context) (domain.Server, bool, error) {
	return domain.Server{}, false, nil
}

func standInExec(t *testing.T, c *podman.Client, script string) {
	t.Helper()
	code, out, err := c.Exec(context.Background(), standIn, []string{"sh", "-c", script})
	if err != nil || code != 0 {
		t.Fatalf("%s: %d %s %v", script, code, out, err)
	}
}

func TestValidateStandIn(t *testing.T) {
	if c, err := net.DialTimeout("tcp", "127.0.0.1:4972", time.Second); err != nil {
		t.Skip("remote stand-in not running (task remote:up)")
	} else {
		c.Close()
	}
	local := podman.New(podman.DefaultSocket())
	ctx := context.Background()
	s := app.NewService(&oneServer{servers: map[uint64]domain.Server{}}, NewServerKey, Connector{Local: local, LocalSocket: podman.DefaultSocket()})
	srv, err := s.Add(ctx, domain.Input{Name: "stand-in", Host: "127.0.0.1", Port: 4972, User: "podman"})
	if err != nil {
		t.Fatal(err)
	}
	standInExec(t, local, "rm -f /etc/bakery-stand-in/no-linger; echo '"+srv.Key.Public+"' >> /home/podman/.ssh/authorized_keys")
	t.Cleanup(func() { standInExec(t, local, "rm -f /etc/bakery-stand-in/no-linger") })

	srv, err = s.Validate(ctx, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if srv.Status != domain.Reachable || srv.HostKey == "" {
		t.Fatalf("%s %+v", srv.Status, srv.Validation)
	}
	for _, c := range srv.Validation.Checks {
		t.Logf("%s ok=%v required=%v: %s", c.Name, c.OK, c.Required, c.Detail)
	}

	m, err := s.Metrics(ctx, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("remote: %+v", m.Server)
	if m.Server.MemTotal <= 0 || m.Server.DiskTotal <= 0 || m.Server.CPUs <= 0 {
		t.Fatalf("remote metrics %+v", m.Server)
	}

	standInExec(t, local, "touch /etc/bakery-stand-in/no-linger")
	srv, err = s.Validate(ctx, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if srv.Status != domain.Unreachable {
		t.Fatalf("no linger: %s %+v", srv.Status, srv.Validation)
	}

	// A different pinned key is refused.
	srv.HostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"
	if _, err := (Connector{}).Connect(ctx, srv); err == nil || !strings.Contains(err.Error(), "host key changed") {
		t.Fatalf("mismatch: %v", err)
	}
}

func TestValidateLocal(t *testing.T) {
	if _, err := os.Stat(podman.DefaultSocket()); err != nil {
		t.Skip("no podman socket")
	}
	local := podman.New(podman.DefaultSocket())
	store := &oneServer{servers: map[uint64]domain.Server{}}
	s := app.NewService(store, NewServerKey, Connector{Local: local, LocalSocket: podman.DefaultSocket()})
	srv, _ := s.EnsureLocal(context.Background())
	srv, err := s.Validate(context.Background(), srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range srv.Validation.Checks {
		t.Logf("%s ok=%v required=%v: %s", c.Name, c.OK, c.Required, c.Detail)
	}
	if srv.Status != domain.Reachable {
		t.Fatalf("%s %+v", srv.Status, srv.Validation)
	}
	m, err := s.Metrics(context.Background(), srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("local: %+v", m.Server)
	for _, c := range m.Containers {
		t.Logf("  %s %s/%s cpu=%.2f mem=%d", c.Name, c.Owner, c.OwnerID, c.CPUPercent, c.MemUsed)
	}
	if m.Server.MemTotal <= 0 || m.Server.DiskTotal <= 0 || m.Server.CPUs <= 0 || m.Server.CPUPercent <= 0 {
		t.Fatalf("local metrics %+v", m.Server)
	}
}

func TestPoolStandIn(t *testing.T) {
	if c, err := net.DialTimeout("tcp", "127.0.0.1:4972", time.Second); err != nil {
		t.Skip("remote stand-in not running (task remote:up)")
	} else {
		c.Close()
	}
	local := podman.New(podman.DefaultSocket())
	ctx := context.Background()
	s := app.NewService(&oneServer{servers: map[uint64]domain.Server{}}, NewServerKey, Connector{Local: local, LocalSocket: podman.DefaultSocket()})
	srv, err := s.Add(ctx, domain.Input{Name: "stand-in", Host: "127.0.0.1", Port: 4972, User: "podman"})
	if err != nil {
		t.Fatal(err)
	}
	standInExec(t, local, "echo '"+srv.Key.Public+"' >> /home/podman/.ssh/authorized_keys")
	if srv, err = s.Validate(ctx, srv.ID); err != nil || srv.HostKey == "" {
		t.Fatalf("%+v %v", srv.Validation, err)
	}

	pool := &Pool{Local: local}
	first, err := pool.Reach(ctx, srv)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Podman.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := pool.Reach(ctx, srv)
	if err != nil || again.Podman != first.Podman {
		t.Fatalf("not reused: %v", err)
	}

	// A connection that died underneath is replaced.
	pool.conns[srv.ID].ssh.Close()
	fresh, err := pool.Reach(ctx, srv)
	if err != nil || fresh.Podman == first.Podman {
		t.Fatalf("not redialled: %v", err)
	}
	if err := fresh.Podman.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	sock, err := pool.conns[srv.ID].ssh.Socket(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := fresh.DialUnix(ctx, sock)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	pool.Forget(srv.ID)
	if _, ok := pool.conns[srv.ID]; ok {
		t.Fatal("still pooled after Forget")
	}

	// A pinned key the Server does not present is refused.
	srv.HostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"
	var changed *app.HostKeyChangedError
	if _, err := pool.Reach(ctx, srv); !errors.As(err, &changed) {
		t.Fatalf("mismatch: %v", err)
	}
}
