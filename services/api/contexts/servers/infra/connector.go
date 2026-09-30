package infra

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// Connector reaches the Local server through Bakery's own Podman socket and
// Remote servers over SSH.
type Connector struct {
	// Local is the client for the Local server's socket, at LocalSocket.
	Local       *podman.Client
	LocalSocket string
}

func (c Connector) Connect(ctx context.Context, s domain.Server) (app.Connection, error) {
	if s.Kind == domain.Local {
		return localConnection{client: c.Local, socket: c.LocalSocket}, nil
	}
	conn, err := podman.DialSSH(ctx, podman.SSHTarget{
		Host: s.Host, Port: s.Port, User: s.User,
		PrivateKey: []byte(s.Key.Private), HostKey: s.HostKey,
	})
	var mismatch *podman.HostKeyMismatchError
	if errors.As(err, &mismatch) {
		return nil, &app.HostKeyChangedError{Detail: mismatch.Error()}
	}
	if err != nil {
		return nil, err
	}
	return &remoteConnection{ssh: conn, client: conn.Podman(), user: s.User}, nil
}

// inContainer reports whether Bakery runs in a Podman container (as the API
// image does on a server), where the host's linger and sysctls are not
// visible.
func inContainer() bool {
	_, err := os.Stat("/run/.containerenv")
	return err == nil
}

type localConnection struct {
	client *podman.Client
	socket string
}

func (l localConnection) Socket(context.Context) (string, error) { return l.socket, nil }
func (l localConnection) Ping(ctx context.Context) error         { return l.client.Ping(ctx) }
func (l localConnection) PodmanVersion(ctx context.Context) (string, error) {
	return l.client.Version(ctx)
}
func (l localConnection) HostKey() string { return "" }
func (l localConnection) Close() error    { return nil }

func (l localConnection) Linger(context.Context) (bool, bool, error) {
	if inContainer() {
		return false, false, nil
	}
	u, err := user.Current()
	if err != nil {
		return false, false, err
	}
	_, err = os.Stat(filepath.Join("/var/lib/systemd/linger", u.Username))
	if errors.Is(err, os.ErrNotExist) {
		return false, true, nil
	}
	return err == nil, err == nil, err
}

func (l localConnection) UnprivilegedPortStart(context.Context) (int, bool, error) {
	if inContainer() {
		return 0, false, nil
	}
	raw, err := os.ReadFile("/proc/sys/net/ipv4/ip_unprivileged_port_start")
	if err != nil {
		return 0, false, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	return n, err == nil, err
}

type remoteConnection struct {
	ssh    *podman.SSHConn
	client *podman.Client
	user   string
}

func (r *remoteConnection) Socket(ctx context.Context) (string, error) { return r.ssh.Socket(ctx) }
func (r *remoteConnection) Ping(ctx context.Context) error             { return r.client.Ping(ctx) }
func (r *remoteConnection) PodmanVersion(ctx context.Context) (string, error) {
	return r.client.Version(ctx)
}
func (r *remoteConnection) HostKey() string { return r.ssh.HostKey() }
func (r *remoteConnection) Close() error    { return r.ssh.Close() }

func (r *remoteConnection) Linger(ctx context.Context) (bool, bool, error) {
	// The user name was validated by the domain (lowercase, digits, _ and -).
	out, err := r.ssh.Run(ctx, "loginctl show-user "+r.user+" -p Linger --value")
	if err != nil {
		return false, false, err
	}
	return strings.TrimSpace(out) == "yes", true, nil
}

func (r *remoteConnection) UnprivilegedPortStart(ctx context.Context) (int, bool, error) {
	out, err := r.ssh.Run(ctx, "cat /proc/sys/net/ipv4/ip_unprivileged_port_start")
	if err != nil {
		return 0, false, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	return n, err == nil, err
}
