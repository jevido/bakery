package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// Connection is an open way to one Server's Podman and shell. Close it.
type Connection interface {
	// Socket is the Podman API socket path on the Server.
	Socket(ctx context.Context) (string, error)
	Ping(ctx context.Context) error
	PodmanVersion(ctx context.Context) (string, error)
	// Linger reports whether the user's Containers survive logout. known
	// is false when it cannot be seen from where Bakery runs.
	Linger(ctx context.Context) (on, known bool, err error)
	// UnprivilegedPortStart is net.ipv4.ip_unprivileged_port_start; known
	// as for Linger.
	UnprivilegedPortStart(ctx context.Context) (port int, known bool, err error)
	// HostKey is the SSH host key presented (empty for the Local server).
	HostKey() string
	Observer
	Close() error
}

// Connector opens Connections.
type Connector interface {
	Connect(ctx context.Context, s domain.Server) (Connection, error)
}

// HostKeyChangedError is what a Connector returns when a Remote server
// presents another host key than the pinned one.
type HostKeyChangedError struct{ Detail string }

func (e *HostKeyChangedError) Error() string { return e.Detail }

// MinPodmanVersion is the oldest Podman Bakery runs on (4.4: the libpod API
// with everything Bakery uses and Quadlet).
const MinPodmanVersion = "4.4"

// validateTimeout bounds one Validation.
const validateTimeout = 30 * time.Second

// Validate connects to the Server, runs the checks and records the outcome.
// A Server that cannot be reached is recorded unreachable; the error is only
// for Bakery's own failures (the database).
func (s *Service) Validate(ctx context.Context, id uint64) (domain.Server, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return srv, err
	}
	vctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()
	checks, hostKey := s.runChecks(vctx, srv)
	srv.RecordValidation(domain.Validation{Checks: checks, CheckedAt: s.now()}, hostKey)
	return srv, s.store.Save(ctx, srv)
}

func (s *Service) runChecks(ctx context.Context, srv domain.Server) ([]domain.Check, string) {
	var checks []domain.Check
	conn, err := s.connector.Connect(ctx, srv)
	if srv.Kind == domain.Remote {
		c := domain.Check{Name: domain.CheckSSH, Required: true, OK: err == nil}
		var changed *HostKeyChangedError
		switch {
		case errors.As(err, &changed):
			c.Detail = changed.Detail + ". If the server was reinstalled or its keys were replaced on purpose, forget the host key and validate again."
		case err != nil:
			c.Detail = err.Error()
		default:
			c.Detail = fmt.Sprintf("connected to %s@%s:%d", srv.User, srv.Host, srv.Port)
		}
		checks = append(checks, c)
	} else if err != nil {
		checks = append(checks, domain.Check{Name: domain.CheckSocket, Required: true, Detail: err.Error()})
	}
	if err != nil {
		return checks, ""
	}
	defer conn.Close()

	socket := domain.Check{Name: domain.CheckSocket, Required: true}
	path, err := conn.Socket(ctx)
	if err == nil {
		err = conn.Ping(ctx)
	}
	if err != nil {
		socket.Detail = fmt.Sprintf("the Podman API socket %s does not answer: %v. Enable it with: systemctl --user enable --now podman.socket", orUnknown(path), err)
		return append(checks, socket), conn.HostKey()
	}
	socket.OK, socket.Detail = true, path+" answers"
	checks = append(checks, socket)

	pod := domain.Check{Name: domain.CheckPodman, Required: true}
	if v, err := conn.PodmanVersion(ctx); err != nil {
		pod.Detail = err.Error()
	} else if !versionAtLeast(v, MinPodmanVersion) {
		pod.Detail = fmt.Sprintf("Podman %s is older than %s; upgrade Podman", v, MinPodmanVersion)
	} else {
		pod.OK, pod.Detail = true, "Podman "+v
	}
	checks = append(checks, pod)

	linger := domain.Check{Name: domain.CheckLinger, Required: srv.Kind == domain.Remote}
	switch on, known, err := conn.Linger(ctx); {
	case err != nil:
		linger.Detail = err.Error()
	case !known:
		linger.OK, linger.Detail = true, "not visible from inside the API container; the install script turns it on"
	case on:
		linger.OK, linger.Detail = true, "on: Containers keep running after logout"
	default:
		user := srv.User
		if user == "" {
			user = "$(whoami)"
		}
		linger.Detail = "off: the user's Containers stop when their last session ends. Turn it on with: sudo loginctl enable-linger " + user
	}
	checks = append(checks, linger)

	ports := domain.Check{Name: domain.CheckPorts}
	switch port, known, err := conn.UnprivilegedPortStart(ctx); {
	case err != nil:
		ports.Detail = err.Error()
	case !known:
		ports.OK, ports.Detail = true, "not visible from inside the API container; the install script sets it"
	case port <= 80:
		ports.OK, ports.Detail = true, fmt.Sprintf("unprivileged ports start at %d: the proxy can listen on 80 and 443", port)
	default:
		ports.Detail = fmt.Sprintf("unprivileged ports start at %d, so a proxy on this server cannot listen on 80 and 443. Fix with: echo net.ipv4.ip_unprivileged_port_start=80 | sudo tee /etc/sysctl.d/90-bakery.conf && sudo sysctl --system", port)
	}
	checks = append(checks, ports)
	return checks, conn.HostKey()
}

func orUnknown(s string) string {
	if s == "" {
		return "(unknown)"
	}
	return s
}

// versionAtLeast compares the major.minor of dotted versions; anything
// unparsable is too old.
func versionAtLeast(v, min string) bool {
	parse := func(s string) (int, int, bool) {
		parts := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(s), "v"), ".", 3)
		if len(parts) < 2 {
			return 0, 0, false
		}
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(strings.TrimRightFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' }))
		return major, minor, err1 == nil && err2 == nil
	}
	a, b, ok := parse(v)
	c, d, _ := parse(min)
	return ok && (a > c || (a == c && b >= d))
}
