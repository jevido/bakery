//go:build podman

package podman

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Runs against the real rootless socket: go test -tags podman ./...

func client(t *testing.T) *Client {
	t.Helper()
	sock := DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	return New(sock)
}

func TestLifecycle(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := c.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := c.EnsureNetwork(ctx, "bakery-test"); err != nil {
			t.Fatalf("EnsureNetwork: %v", err)
		}
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM docker.io/library/busybox\nRUN echo building\nCMD [\"sh\", \"-c\", \"echo hi; echo to-stderr >&2; echo $GREETING\"]\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o644)

	var buildLog []string
	tag := "localhost/bakery-test/lifecycle:1"
	imageID, err := c.Build(ctx, TarDir(dir), BuildOptions{Tag: tag, Dockerfile: "Containerfile"}, func(l string) { buildLog = append(buildLog, l) })
	if err != nil {
		t.Fatalf("Build: %v\n%s", err, strings.Join(buildLog, "\n"))
	}
	if imageID == "" || len(buildLog) == 0 || !strings.Contains(strings.Join(buildLog, "\n"), "STEP") {
		t.Fatalf("build: id %q, log %v", imageID, buildLog)
	}
	if ok, err := c.ImageExists(ctx, tag); err != nil || !ok {
		t.Fatalf("ImageExists: %v %v", ok, err)
	}

	id, err := c.CreateContainer(ctx, ContainerSpec{
		Name:  "bakery-test-lifecycle",
		Image: tag,
		Env:   map[string]string{"GREETING": "hello-env"},
		// Its own label value: other packages' podman tests run at the same
		// time with bakery.test=true containers of their own.
		Labels:   map[string]string{"bakery.test": "lifecycle"},
		Networks: OnNetwork("bakery-test"),
	})
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	defer c.RemoveContainer(context.Background(), id)
	if err := c.StartContainer(ctx, id); err != nil {
		t.Fatalf("StartContainer: %v", err)
	}

	list, err := c.ListContainers(ctx, map[string]string{"bakery.test": "lifecycle"})
	if err != nil || len(list) != 1 {
		t.Fatalf("ListContainers: %v %v", list, err)
	}

	var lines []string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		lines = nil
		if err := c.Logs(ctx, id, false, -1, func(s, l string) { lines = append(lines, s+":"+l) }); err != nil {
			t.Fatalf("Logs: %v", err)
		}
		if len(lines) >= 3 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	got := strings.Join(lines, ",")
	for _, want := range []string{"stdout:hi", "stderr:to-stderr", "stdout:hello-env"} {
		if !strings.Contains(got, want) {
			t.Fatalf("logs %q lack %q", got, want)
		}
	}
	info, err := c.InspectContainer(ctx, id)
	if err != nil || info.Config.Labels["bakery.test"] != "lifecycle" {
		t.Fatalf("Inspect: %+v %v", info, err)
	}
	if err := c.RemoveContainer(ctx, id); err != nil {
		t.Fatalf("RemoveContainer: %v", err)
	}
	if _, err := c.InspectContainer(ctx, id); !IsNotFound(err) {
		t.Fatalf("after remove: %v", err)
	}
}

func TestCopyInto(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	image := "docker.io/library/busybox:latest"
	if ok, _ := c.ImageExists(ctx, image); !ok {
		if err := c.PullImage(ctx, image, nil); err != nil {
			t.Fatal(err)
		}
	}
	id, err := c.CreateContainer(ctx, ContainerSpec{
		Name: "bakery-test-copy", Image: image, Command: []string{"sleep", "60"},
		Labels: map[string]string{"bakery.test": "copy"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.RemoveContainer(context.Background(), id)

	want := []byte("-----BEGIN CERTIFICATE-----\ntest\n")
	if err := c.CopyInto(ctx, id, "/tmp", map[string][]byte{"bakery/nested/root.pem": want}); err != nil {
		t.Fatalf("CopyInto: %v", err)
	}
	got, err := c.ReadFile(ctx, id, "/tmp/bakery/nested/root.pem")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("read back %q, want %q", got, want)
	}
}

func busybox(t *testing.T, ctx context.Context, c *Client) string {
	t.Helper()
	image := "docker.io/library/busybox:latest"
	if ok, _ := c.ImageExists(ctx, image); !ok {
		if err := c.PullImage(ctx, image, nil); err != nil {
			t.Fatal(err)
		}
	}
	return image
}

func TestExec(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id, err := c.CreateContainer(ctx, ContainerSpec{
		Name: "bakery-test-exec", Image: busybox(t, ctx, c), Command: []string{"sleep", "60"},
		Labels: map[string]string{"bakery.managed": "true", "bakery.test": "exec"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.RemoveContainer(context.Background(), id)
	if err := c.StartContainer(ctx, id); err != nil {
		t.Fatal(err)
	}
	code, out, err := c.Exec(ctx, id, []string{"sh", "-c", "echo hi; echo oops >&2; exit 3"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if code != 3 || out != "hi\noops" {
		t.Fatalf("Exec: code %d, output %q", code, out)
	}
	if code, _, err := c.Exec(ctx, id, []string{"true"}); err != nil || code != 0 {
		t.Fatalf("Exec true: %d %v", code, err)
	}
	// A command the image does not have fails, not hangs.
	if code, _, err := c.Exec(ctx, id, []string{"no-such-command"}); err == nil && code == 0 {
		t.Fatalf("Exec of a missing command succeeded")
	}
}

func TestBuildArgs(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	busybox(t, ctx, c)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM docker.io/library/busybox:latest\nARG GREETING\nRUN test \"$GREETING\" = hello\n"), 0o644)
	tag := "localhost/bakery-test/buildargs:1"
	defer c.RemoveImage(context.Background(), tag)

	if _, err := c.Build(ctx, TarDir(dir), BuildOptions{Tag: tag, Dockerfile: "Containerfile"}, func(string) {}); err == nil {
		t.Fatal("build without the build arg passed")
	}
	if _, err := c.Build(ctx, TarDir(dir), BuildOptions{Tag: tag, Dockerfile: "Containerfile", BuildArgs: map[string]string{"GREETING": "hello"}}, func(string) {}); err != nil {
		t.Fatalf("build with the build arg: %v", err)
	}
	if err := c.RemoveImage(ctx, tag); err != nil {
		t.Fatalf("RemoveImage: %v", err)
	}
	if ok, _ := c.ImageExists(ctx, tag); ok {
		t.Fatal("image still there after RemoveImage")
	}
}

func TestPullTagDigest(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Not Docker Hub: pulls there always go to the registry and are
	// rate limited.
	id, err := c.PullImageWith(ctx, "ghcr.io/traefik/whoami:v1.10", PullOptions{TLSVerify: true}, nil)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if id == "" {
		t.Fatal("pull returned no image id")
	}
	const tagged = "localhost/bakery/test-tag:1"
	if err := c.TagImage(ctx, id, "localhost/bakery/test-tag", "1"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	// Untag only our own name.
	defer c.call(context.Background(), "POST", "/images/"+id+"/untag", map[string][]string{"repo": {"localhost/bakery/test-tag"}, "tag": {"1"}}, nil, nil)
	if ok, err := c.ImageExists(ctx, tagged); err != nil || !ok {
		t.Fatalf("tagged image exists = %v, %v", ok, err)
	}
	digest, err := c.ImageDigest(ctx, tagged)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if !strings.Contains(digest, "@sha256:") {
		t.Fatalf("digest %q has no @sha256:", digest)
	}

	if _, err := c.PullImageWith(ctx, "ghcr.io/jevido/bakery-does-not-exist:1", PullOptions{TLSVerify: true}, nil); err == nil {
		t.Fatal("pulling a missing image succeeded")
	}
}

func TestVolumesAndLimits(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	vol := "bakery-test-volume-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	labels := map[string]string{"bakery.managed": "true", "bakery.test": vol}
	for range 2 {
		if err := c.CreateVolume(ctx, vol, labels); err != nil {
			t.Fatalf("CreateVolume: %v", err)
		}
	}
	defer c.RemoveVolume(context.Background(), vol)
	list, err := c.ListVolumes(ctx, map[string]string{"bakery.test": vol})
	if err != nil || len(list) != 1 || list[0].Name != vol || list[0].Labels["bakery.managed"] != "true" {
		t.Fatalf("ListVolumes: %+v %v", list, err)
	}

	if err := c.PullImage(ctx, "docker.io/library/busybox", func(string) {}); err != nil {
		t.Fatal(err)
	}
	name := vol + "-ctr"
	_, err = c.CreateContainer(ctx, ContainerSpec{
		Name: name, Image: "docker.io/library/busybox", Command: []string{"sleep", "60"},
		Labels:         map[string]string{"bakery.managed": "true"},
		Volumes:        []NamedVolume{{Name: vol, Dest: "/data"}},
		ResourceLimits: Limits(64, 0.5),
	})
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	defer c.RemoveContainer(context.Background(), name)
	info, err := c.InspectContainer(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if info.HostConfig.Memory != 64<<20 || info.HostConfig.CPUQuota != 50000 {
		t.Fatalf("limits: memory %d, cpu quota %d", info.HostConfig.Memory, info.HostConfig.CPUQuota)
	}
	if len(info.Mounts) != 1 || info.Mounts[0].Name != vol || info.Mounts[0].Destination != "/data" {
		t.Fatalf("mounts: %+v", info.Mounts)
	}
	if err := c.StartContainer(ctx, name); err != nil {
		t.Fatalf("start with limits: %v", err)
	}

	if err := c.RemoveContainer(ctx, name); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveVolume(ctx, vol); err != nil {
		t.Fatal(err)
	}
	if list, _ := c.ListVolumes(ctx, map[string]string{"bakery.test": vol}); len(list) != 0 {
		t.Fatalf("volume still there: %+v", list)
	}
}
