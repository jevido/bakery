//go:build podman

package infra

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// Runs a throwaway proxy (bakery-test-proxy on 4944/4945/4946) and a whoami
// container, and checks a Route reaches it over HTTPS with internal TLS.
func TestProxyRoutesToContainer(t *testing.T) {
	pc := podman.New(podman.DefaultSocket())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := pc.Ping(ctx); err != nil {
		t.Skipf("no podman: %v", err)
	}

	proxy := NewProxy(pc, ProxyConfig{
		Name: "bakery-test-proxy", Image: "docker.io/library/caddy:2", Network: "bakery-test",
		BindIP: "127.0.0.1", HTTPPort: 4944, HTTPSPort: 4945, AdminURL: "http://127.0.0.1:4946", AdminPublish: "127.0.0.1:4946",
		InternalTLS: true, VolumePrefix: "bakery-test-proxy",
	})
	t.Cleanup(func() {
		bg := context.Background()
		pc.RemoveContainer(bg, "bakery-test-proxy")
		pc.RemoveVolume(bg, "bakery-test-proxy-data")
		pc.RemoveVolume(bg, "bakery-test-proxy-config")
	})
	for range 2 { // the second Ensure must find and keep the first container
		if err := proxy.Ensure(ctx); err != nil {
			t.Fatalf("Ensure: %v", err)
		}
	}
	list, _ := pc.ListContainers(ctx, map[string]string{"bakery.role": "proxy"})
	n := 0
	for _, c := range list {
		if strings.Contains(strings.Join(c.Names, ","), "bakery-test-proxy") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one test proxy, found %d", n)
	}

	image := "docker.io/traefik/whoami:latest"
	if ok, _ := pc.ImageExists(ctx, image); !ok {
		if err := pc.PullImage(ctx, image, nil); err != nil {
			t.Fatal(err)
		}
	}
	id, err := pc.CreateContainer(ctx, podman.ContainerSpec{
		Name: "bakery-test-whoami", Image: image, Networks: podman.OnNetwork("bakery-test"),
		Labels: map[string]string{"bakery.test": "true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.RemoveContainer(context.Background(), id) })
	if err := pc.StartContainer(ctx, id); err != nil {
		t.Fatal(err)
	}

	if err := proxy.Apply(ctx, []domain.Route{{ApplicationID: 1, Domain: "test.localhost", Container: "bakery-test-whoami", Port: 80}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// curl -k --resolve test.localhost:4945:127.0.0.1 https://test.localhost:4945
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:4945")
		},
	}}
	var body string
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		res, err := client.Get("https://test.localhost:4945/")
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if body = string(raw); res.StatusCode == 200 && strings.Contains(body, "Hostname:") {
			return
		}
	}
	t.Fatalf("whoami never answered through the proxy; last body %q", body)
}

// With AdminPublish empty the admin API gets no host port; the API reaches
// it over the network instead (here: not at all, so only Create is checked).
func TestProxyAdminNotPublished(t *testing.T) {
	pc := podman.New(podman.DefaultSocket())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := pc.Ping(ctx); err != nil {
		t.Skipf("no podman: %v", err)
	}
	if err := pc.EnsureNetwork(ctx, "bakery-test"); err != nil {
		t.Fatal(err)
	}
	proxy := NewProxy(pc, ProxyConfig{
		Name: "bakery-test-proxy-unpublished", Image: "docker.io/library/caddy:2", Network: "bakery-test",
		BindIP: "127.0.0.1", HTTPPort: 4947, HTTPSPort: 4948, AdminURL: "http://bakery-test-proxy-unpublished:2019",
		InternalTLS: true, VolumePrefix: "bakery-test-proxy-unpublished",
	})
	t.Cleanup(func() {
		bg := context.Background()
		pc.RemoveContainer(bg, "bakery-test-proxy-unpublished")
		pc.RemoveVolume(bg, "bakery-test-proxy-unpublished-data")
		pc.RemoveVolume(bg, "bakery-test-proxy-unpublished-config")
	})
	if err := proxy.create(ctx); err != nil {
		t.Fatalf("create: %v", err)
	}
	info, err := pc.InspectContainer(ctx, "bakery-test-proxy-unpublished")
	if err != nil {
		t.Fatal(err)
	}
	if b := info.HostConfig.PortBindings["2019/tcp"]; len(b) != 0 {
		t.Fatalf("admin API published on the host: %+v", b)
	}
	if b := info.HostConfig.PortBindings["443/tcp"]; len(b) != 1 || b[0].HostPort != "4948" {
		t.Fatalf("want 443 on host 4948, got %+v", info.HostConfig.PortBindings)
	}
}
