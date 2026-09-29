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
		BindIP: "127.0.0.1", HTTPPort: 4944, HTTPSPort: 4945, AdminAddr: "127.0.0.1:4946",
		InternalTLS: true, VolumePrefix: "bakery-test-proxy",
	})
	t.Cleanup(func() { pc.RemoveContainer(context.Background(), "bakery-test-proxy") })
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
