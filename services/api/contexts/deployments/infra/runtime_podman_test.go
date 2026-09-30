//go:build podman

package infra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
)

// buildTestImage builds a busybox httpd image serving /health; with noWget
// the image has neither curl nor wget.
func buildTestImage(t *testing.T, ctx context.Context, c *podman.Client, tag string, noWget bool) {
	t.Helper()
	dir := t.TempDir()
	file := "FROM docker.io/library/busybox:latest\nRUN mkdir /www && echo ok > /www/health\n"
	if noWget {
		file += "RUN rm /bin/wget\n"
	}
	file += `CMD ["httpd", "-f", "-p", "8080", "-h", "/www"]` + "\n"
	os.WriteFile(filepath.Join(dir, "Containerfile"), []byte(file), 0o644)
	if _, err := c.Build(ctx, podman.TarDir(dir), podman.BuildOptions{Tag: tag, Dockerfile: "Containerfile"}, func(string) {}); err != nil {
		t.Fatalf("build %s: %v", tag, err)
	}
	t.Cleanup(func() { c.RemoveImage(context.Background(), tag) })
}

func TestProbe(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	c := podman.New(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := c.EnsureNetwork(ctx, "bakery-test"); err != nil {
		t.Fatal(err)
	}
	r := Runtime{Podman: c, Network: "bakery-test", StartTimeout: 20 * time.Second, Settle: time.Second}
	buildTestImage(t, ctx, c, "localhost/bakery-test/probe:1", false)
	buildTestImage(t, ctx, c, "localhost/bakery-test/probe-bare:1", true)

	const appID = 990001
	defer r.RemoveAll(context.Background(), appID)
	if err := r.Start(ctx, app.ContainerSpec{Name: "bakery-test-probe", Image: "localhost/bakery-test/probe:1", ApplicationID: appID, DeploymentID: 1}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	var ok bool
	var detail string
	var err error
	// httpd may need a moment to listen.
	for range 20 {
		if ok, detail, err = r.Probe(ctx, "bakery-test-probe", "http://127.0.0.1:8080/health", 2*time.Second); ok || err != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ok || err != nil {
		t.Fatalf("healthy path: ok %v, detail %q, err %v", ok, detail, err)
	}
	ok, detail, err = r.Probe(ctx, "bakery-test-probe", "http://127.0.0.1:8080/missing", 2*time.Second)
	if ok || err != nil || !strings.Contains(detail, "404") {
		t.Fatalf("missing path: ok %v, detail %q, err %v", ok, detail, err)
	}

	if err := r.Start(ctx, app.ContainerSpec{Name: "bakery-test-probe-bare", Image: "localhost/bakery-test/probe-bare:1", ApplicationID: appID, DeploymentID: 2}); err != nil {
		t.Fatalf("Start bare: %v", err)
	}
	if _, _, err := r.Probe(ctx, "bakery-test-probe-bare", "http://127.0.0.1:8080/health", 2*time.Second); err == nil || !strings.Contains(err.Error(), "curl or wget") {
		t.Fatalf("image without a tool: %v", err)
	}
}

func TestPull(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	c := podman.New(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := Runtime{Podman: c}
	const tag = "localhost/bakery/test-pull:1"
	t.Cleanup(func() { c.RemoveImage(context.Background(), tag) })
	digest, err := r.Pull(ctx, app.PullRequest{Reference: "ghcr.io/traefik/whoami:v1.10", Tag: tag}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(digest, "ghcr.io/traefik/whoami@sha256:") {
		t.Errorf("digest %q", digest)
	}
	if ok, _ := c.ImageExists(ctx, tag); !ok {
		t.Errorf("%s not tagged", tag)
	}
	if RegistryHost("127.0.0.1:4950/me/app:1") != "127.0.0.1:4950" {
		t.Error("RegistryHost")
	}
}
