package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

type fakeConnection struct {
	pingErr     error
	version     string
	linger      bool
	lingerKnown bool
	port        int
	hostKey     string
	cpuErr      error
	pruned      int
	closed      bool
	// free is the free disk space; 0 means 40 GiB of 100.
	free int64
}

func (f *fakeConnection) Socket(context.Context) (string, error) {
	return "/run/user/1000/podman/podman.sock", nil
}
func (f *fakeConnection) Ping(context.Context) error { return f.pingErr }
func (f *fakeConnection) PodmanVersion(context.Context) (string, error) {
	return f.version, nil
}
func (f *fakeConnection) Linger(context.Context) (bool, bool, error) {
	return f.linger, f.lingerKnown, nil
}
func (f *fakeConnection) UnprivilegedPortStart(context.Context) (int, bool, error) {
	return f.port, true, nil
}
func (f *fakeConnection) HostKey() string { return f.hostKey }
func (f *fakeConnection) PruneDanglingImages(context.Context) (int64, error) {
	f.pruned++
	return 10, nil
}
func (f *fakeConnection) HostInfo(context.Context) (HostInfo, error) {
	return HostInfo{CPUs: 4, MemTotal: 8 << 30, MemFree: 2 << 30, GraphRoot: "/var/lib/containers"}, nil
}
func (f *fakeConnection) CPUPercent(context.Context) (float64, error) {
	if f.cpuErr != nil {
		return 0, f.cpuErr
	}
	return 25, nil
}
func (f *fakeConnection) Filesystem(context.Context, string) (int64, int64, error) {
	if f.free != 0 {
		return 100 << 30, f.free, nil
	}
	return 100 << 30, 40 << 30, nil
}
func (f *fakeConnection) PodmanDiskUsage(context.Context) (PodmanDiskUsage, error) {
	return PodmanDiskUsage{Images: 5 << 30, Volumes: 1 << 30}, nil
}
func (f *fakeConnection) Containers(context.Context) ([]ContainerSample, error) {
	return []ContainerSample{
		{Name: "bakery-proxy", Labels: map[string]string{"bakery.role": "proxy"}, CPU: 1, MemUsage: 30 << 20},
		{Name: "bakery-app-3-7", Labels: map[string]string{"bakery.application": "3"}, CPU: 2, MemUsage: 60 << 20, MemLimit: 512 << 20},
	}, nil
}
func (f *fakeConnection) Close() error { f.closed = true; return nil }

type fakeConnector struct {
	conn *fakeConnection
	err  error
}

func (f fakeConnector) Connect(context.Context, domain.Server) (Connection, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.conn, nil
}

func healthy() *fakeConnection {
	return &fakeConnection{version: "5.8.7", linger: true, lingerKnown: true, port: 80, hostKey: "ssh-ed25519 HOST"}
}

func validate(t *testing.T, c fakeConnector) domain.Server {
	t.Helper()
	s := NewService(newMemStore(), fakeKey, c)
	srv, err := s.Add(context.Background(), domain.Input{Name: "web", Host: "10.0.0.1", User: "bakery"})
	if err != nil {
		t.Fatal(err)
	}
	srv, err = s.Validate(context.Background(), srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func check(t *testing.T, srv domain.Server, name string) domain.Check {
	t.Helper()
	for _, c := range srv.Validation.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s check in %+v", name, srv.Validation.Checks)
	return domain.Check{}
}

func TestValidateHealthy(t *testing.T) {
	conn := healthy()
	srv := validate(t, fakeConnector{conn: conn})
	if srv.Status != domain.Reachable || srv.HostKey != "ssh-ed25519 HOST" || !conn.closed {
		t.Fatalf("%+v closed=%v", srv, conn.closed)
	}
	if len(srv.Validation.Checks) != 5 || srv.Validation.CheckedAt.IsZero() {
		t.Fatalf("%+v", srv.Validation)
	}
}

func TestValidateUnreachable(t *testing.T) {
	srv := validate(t, fakeConnector{err: errors.New("ssh: dial tcp 10.0.0.1:22: connection refused")})
	if srv.Status != domain.Unreachable || srv.HostKey != "" {
		t.Fatalf("%+v", srv)
	}
	if c := check(t, srv, domain.CheckSSH); c.OK || !strings.Contains(c.Detail, "refused") {
		t.Fatalf("%+v", c)
	}
}

func TestValidateHostKeyChanged(t *testing.T) {
	srv := validate(t, fakeConnector{err: &HostKeyChangedError{Detail: "ssh: host key changed"}})
	if c := check(t, srv, domain.CheckSSH); c.OK || !strings.Contains(c.Detail, "forget the host key") {
		t.Fatalf("%+v", c)
	}
}

func TestValidateOldPodman(t *testing.T) {
	conn := healthy()
	conn.version = "4.3.1"
	srv := validate(t, fakeConnector{conn: conn})
	if srv.Status != domain.Unreachable || check(t, srv, domain.CheckPodman).OK {
		t.Fatalf("%+v", srv)
	}
}

func TestValidateNoLinger(t *testing.T) {
	conn := healthy()
	conn.linger = false
	srv := validate(t, fakeConnector{conn: conn})
	c := check(t, srv, domain.CheckLinger)
	if srv.Status != domain.Unreachable || c.OK || !strings.Contains(c.Detail, "enable-linger bakery") {
		t.Fatalf("%+v", srv)
	}
}

func TestValidatePortsNotRequired(t *testing.T) {
	conn := healthy()
	conn.port = 1024
	srv := validate(t, fakeConnector{conn: conn})
	if srv.Status != domain.Reachable || check(t, srv, domain.CheckPorts).OK {
		t.Fatalf("%+v", srv)
	}
}

func TestValidateSocketDown(t *testing.T) {
	conn := healthy()
	conn.pingErr = errors.New("connection refused")
	srv := validate(t, fakeConnector{conn: conn})
	c := check(t, srv, domain.CheckSocket)
	if srv.Status != domain.Unreachable || c.OK || !strings.Contains(c.Detail, "podman.socket") {
		t.Fatalf("%+v", srv)
	}
	// The connection got through, so the host key is pinned.
	if srv.HostKey == "" {
		t.Fatal("host key not pinned")
	}
}

func TestVersionAtLeast(t *testing.T) {
	for v, want := range map[string]bool{"4.4.0": true, "4.10.1": true, "5.0.0": true, "4.3.9": false, "3.9": false, "dev": false, "v4.4": true} {
		if got := versionAtLeast(v, MinPodmanVersion); got != want {
			t.Errorf("%s: %v", v, got)
		}
	}
}
