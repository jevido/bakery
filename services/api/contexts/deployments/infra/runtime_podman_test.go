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

// TestNixpacksBuild plans a tiny Node app with the real nixpacks binary
// (task api:tools) and builds it with Podman.
func TestNixpacksBuild(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	bin := os.Getenv("BAKERY_NIXPACKS")
	if bin == "" {
		bin, _ = filepath.Abs("../../../bin/nixpacks")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("no nixpacks at %s (run task api:tools)", bin)
	}
	c := podman.New(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"hello","version":"1.0.0","scripts":{"start":"node index.js"}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(`require("http").createServer((q, r) => r.end(process.env.GREETING)).listen(3000)`), 0o644)

	file, args, err := Nixpacks{Binary: bin}.Plan(ctx, dir, map[string]string{"GREETING": "hi"}, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if args["NODE_ENV"] != "production" || args["GREETING"] != "hi" {
		t.Errorf("build args %v", args)
	}
	const tag = "localhost/bakery/test-nixpacks:1"
	t.Cleanup(func() { c.RemoveImage(context.Background(), tag) })
	var tail []string
	err = Runtime{Podman: c}.Build(ctx, app.BuildRequest{Dir: dir, Dockerfile: file, Tag: tag, BuildArgs: args,
		Labels: map[string]string{"bakery.managed": "true"}}, func(line string) { tail = append(tail, line) })
	if err != nil {
		t.Fatalf("build: %v\n%s", err, strings.Join(tail[max(0, len(tail)-20):], "\n"))
	}
}

// Two Containers of one Application share a Persistent storage, and
// RemoveVolumes removes only that Application's Volumes.
func TestPersistentStorage(t *testing.T) {
	c := podman.New(podman.DefaultSocket())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		t.Skipf("no podman: %v", err)
	}
	if err := c.EnsureNetwork(ctx, "bakery-test"); err != nil {
		t.Fatal(err)
	}
	r := Runtime{Podman: c, Network: "bakery-test", StartTimeout: 20 * time.Second}
	buildTestImage(t, ctx, c, "localhost/bakery-test/storage:1", false)
	const appID = 990002
	defer r.RemoveVolumes(context.Background(), appID)
	defer r.RemoveAll(context.Background(), appID)

	mounts := []app.Mount{{Volume: "bakery-app-990002-data", Path: "/data"}}
	if err := r.Start(ctx, app.ContainerSpec{Name: "bakery-test-storage-1", Image: "localhost/bakery-test/storage:1", ApplicationID: appID, DeploymentID: 1, Mounts: mounts, MemoryMB: 128, CPUs: 0.5}); err != nil {
		t.Fatal(err)
	}
	if info, err := c.InspectContainer(ctx, "bakery-test-storage-1"); err != nil || info.HostConfig.Memory != 128<<20 || info.HostConfig.CPUQuota != 50000 {
		t.Fatalf("limits: %+v %v", info.HostConfig, err)
	}
	if code, out, err := c.Exec(ctx, "bakery-test-storage-1", []string{"sh", "-c", "echo kept > /data/file"}); err != nil || code != 0 {
		t.Fatalf("write: %d %s %v", code, out, err)
	}
	if err := r.Start(ctx, app.ContainerSpec{Name: "bakery-test-storage-2", Image: "localhost/bakery-test/storage:1", ApplicationID: appID, DeploymentID: 2, Mounts: mounts}); err != nil {
		t.Fatal(err)
	}
	if code, out, err := c.Exec(ctx, "bakery-test-storage-2", []string{"cat", "/data/file"}); err != nil || code != 0 || strings.TrimSpace(out) != "kept" {
		t.Fatalf("read in the second container: %d %q %v", code, out, err)
	}

	if err := r.RemoveAll(ctx, appID); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveVolumes(ctx, appID); err != nil {
		t.Fatal(err)
	}
	if list, _ := c.ListVolumes(ctx, map[string]string{"bakery.application": "990002"}); len(list) != 0 {
		t.Fatalf("volumes left: %+v", list)
	}
}

func TestPreviewContainersApart(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	c := podman.New(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := Runtime{Podman: c, Network: "bakery-test", StartTimeout: 20 * time.Second, Settle: time.Second}
	buildTestImage(t, ctx, c, "localhost/bakery-test/preview:1", false)

	const appID = 990003
	defer r.RemoveAll(context.Background(), appID)
	for _, spec := range []app.ContainerSpec{
		{Name: "bakery-test-prod", Image: "localhost/bakery-test/preview:1", ApplicationID: appID, DeploymentID: 1},
		{Name: "bakery-test-pr7", Image: "localhost/bakery-test/preview:1", ApplicationID: appID, DeploymentID: 2, Preview: 7},
		{Name: "bakery-test-pr8", Image: "localhost/bakery-test/preview:1", ApplicationID: appID, DeploymentID: 3, Preview: 8},
	} {
		if err := r.Start(ctx, spec); err != nil {
			t.Fatalf("Start %s: %v", spec.Name, err)
		}
	}
	if name, found, err := r.Running(ctx, appID); err != nil || !found || name != "bakery-test-prod" {
		t.Fatalf("Running: %q %v %v", name, found, err)
	}
	removed, err := r.RemoveOthers(ctx, appID, 7, "")
	if err != nil || len(removed) != 1 || removed[0] != "bakery-test-pr7" {
		t.Fatalf("RemoveOthers of preview 7: %v %v", removed, err)
	}
	removed, err = r.RemoveOthers(ctx, appID, 0, "bakery-test-prod")
	if err != nil || len(removed) != 0 {
		t.Fatalf("RemoveOthers of production keeping the running one: %v %v", removed, err)
	}
	if err := r.RemoveAll(ctx, appID); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.containers(ctx, appID); len(list) != 0 {
		t.Fatalf("left after RemoveAll: %d", len(list))
	}
}

func TestRemovePreview(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	c := podman.New(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := Runtime{Podman: c, Network: "bakery-test", StartTimeout: 20 * time.Second, Settle: time.Second}
	buildTestImage(t, ctx, c, "localhost/bakery-test/preview-rm:1", false)

	const appID = 990004
	defer func() {
		r.RemoveAll(context.Background(), appID)
		r.RemoveVolumes(context.Background(), appID)
	}()
	for _, spec := range []app.ContainerSpec{
		{Name: "bakery-test-rm-prod", Image: "localhost/bakery-test/preview-rm:1", ApplicationID: appID, DeploymentID: 1,
			Mounts: []app.Mount{{Volume: "bakery-test-rm-data", Path: "/data"}}},
		{Name: "bakery-test-rm-pr7", Image: "localhost/bakery-test/preview-rm:1", ApplicationID: appID, DeploymentID: 2, Preview: 7,
			Mounts: []app.Mount{{Volume: "bakery-test-rm-pr7-data", Path: "/data"}}},
	} {
		if err := r.Start(ctx, spec); err != nil {
			t.Fatalf("Start %s: %v", spec.Name, err)
		}
	}
	start := time.Now()
	removed, err := r.RemovePreview(ctx, appID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(removed, ",") != "bakery-test-rm-pr7,bakery-test-rm-pr7-data" {
		t.Fatalf("removed %v", removed)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("removing took %s; it should not wait for a graceful stop", time.Since(start))
	}
	if name, found, _ := r.Running(ctx, appID); !found || name != "bakery-test-rm-prod" {
		t.Fatalf("production container: %q %v", name, found)
	}
	vols, err := c.ListVolumes(ctx, map[string]string{"bakery.application": "990004"})
	if err != nil || len(vols) != 1 || vols[0].Name != "bakery-test-rm-data" {
		t.Fatalf("volumes left: %+v %v", vols, err)
	}
}

func TestStatesAndStop(t *testing.T) {
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
	buildTestImage(t, ctx, c, "localhost/bakery-test/states:1", false)

	const appID = 990003
	defer r.RemoveAll(context.Background(), appID)
	if states, running, err := r.States(ctx, appID); err != nil || len(states) != 0 || running != "" {
		t.Fatalf("before: %v %q %v", states, running, err)
	}
	for _, spec := range []app.ContainerSpec{
		{Name: "bakery-test-states-1", DeploymentID: 1},
		{Name: "bakery-test-states-2", DeploymentID: 2},
		{Name: "bakery-test-states-pr3", DeploymentID: 3, Preview: 3},
	} {
		spec.Image, spec.ApplicationID = "localhost/bakery-test/states:1", appID
		if err := r.Start(ctx, spec); err != nil {
			t.Fatalf("Start %s: %v", spec.Name, err)
		}
	}
	states, running, err := r.States(ctx, appID)
	if err != nil || len(states) != 2 || states[0] != "running" || running != "bakery-test-states-2" {
		t.Fatalf("running: %v %q %v", states, running, err)
	}
	stopped, err := r.Stop(ctx, appID)
	if err != nil || len(stopped) != 2 {
		t.Fatalf("Stop: %v %v", stopped, err)
	}
	if states, running, err := r.States(ctx, appID); err != nil || len(states) != 0 || running != "" {
		t.Fatalf("after: %v %q %v", states, running, err)
	}
	// The Preview keeps running.
	list, _ := r.containers(ctx, appID)
	if len(list) != 1 || list[0].State != "running" {
		t.Fatalf("preview after stop: %+v", list)
	}
}
