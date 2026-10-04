//go:build podman

package podman

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
	var stamped []string
	if err := c.TimestampedLogs(ctx, id, false, 1, func(_, l string) { stamped = append(stamped, l) }); err != nil {
		t.Fatalf("TimestampedLogs: %v", err)
	}
	if len(stamped) != 1 || !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2}) `).MatchString(stamped[0]) {
		t.Fatalf("timestamped tail 1: %q", stamped)
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
	// Binary output survives ExecStream byte for byte, and so does a file
	// streamed in with CopyFileInto.
	payload := make([]byte, 1<<20)
	for i := range payload {
		payload[i] = byte(i * 7 % 251)
	}
	if err := c.CopyFileInto(ctx, id, "/tmp", "payload.bin", int64(len(payload)), bytes.NewReader(payload)); err != nil {
		t.Fatalf("CopyFileInto: %v", err)
	}
	var got bytes.Buffer
	if code, stderr, err := c.ExecStream(ctx, id, []string{"cat", "/tmp/payload.bin"}, &got); err != nil || code != 0 {
		t.Fatalf("ExecStream cat: %d %q %v", code, stderr, err)
	}
	if !bytes.Equal(got.Bytes(), payload) {
		t.Fatalf("ExecStream: got %d bytes, want %d identical", got.Len(), len(payload))
	}
	got.Reset()
	code, stderr, err := c.ExecStream(ctx, id, []string{"sh", "-c", "echo out; echo broken >&2; exit 3"}, &got)
	if err != nil || code != 3 || stderr != "broken" || got.String() != "out\n" {
		t.Fatalf("ExecStream: code %d, stderr %q, stdout %q, %v", code, stderr, got.String(), err)
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

	name := vol + "-ctr"
	_, err = c.CreateContainer(ctx, ContainerSpec{
		Name: name, Image: busybox(t, ctx, c), Command: []string{"sleep", "60"},
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

// A published port is reachable on the host address, and a sh -c command
// reads a secret from the environment instead of its own command line (how
// Databases pass passwords).
func TestPublishedPort(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := c.EnsureNetwork(ctx, "bakery-test"); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	name := "bakery-test-published"
	c.RemoveContainer(ctx, name)
	_, err = c.CreateContainer(ctx, ContainerSpec{
		Name: name, Image: busybox(t, ctx, c),
		Command:      []string{"sh", "-c", `mkdir -p /www && echo "$SECRET" > /www/index.html && exec httpd -f -p 8080 -h /www`},
		Env:          map[string]string{"SECRET": "from-env"},
		Labels:       map[string]string{"bakery.managed": "true", "bakery.test": "published"},
		Networks:     OnNetwork("bakery-test"),
		PortMappings: []PortMapping{{HostIP: "127.0.0.1", HostPort: uint16(port), ContainerPort: 8080, Protocol: "tcp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.RemoveContainer(context.Background(), name)
	if err := c.StartContainer(ctx, name); err != nil {
		t.Fatal(err)
	}

	info, err := c.InspectContainer(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if b := info.HostConfig.PortBindings["8080/tcp"]; len(b) != 1 || b[0].HostPort != strconv.Itoa(port) || b[0].HostIP != "127.0.0.1" {
		t.Fatalf("port bindings: %+v", info.HostConfig.PortBindings)
	}

	var body string
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) {
		res, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/")
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		body = strings.TrimSpace(string(b))
		break
	}
	if body != "from-env" {
		t.Fatalf("published port answered %q", body)
	}
}

func TestNetworkAliasesAndSpec(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	const network = "bakery-test-aliases"
	labels := map[string]string{"bakery.test": "aliases"}
	names := []string{"bakery-test-aliases-db", "bakery-test-aliases-app"}
	cleanup := func() {
		for _, n := range names {
			c.RemoveContainer(context.Background(), n)
		}
		c.RemoveNetwork(context.Background(), network)
	}
	cleanup()
	t.Cleanup(cleanup)

	if err := c.EnsureNetworkLabeled(ctx, network, labels); err != nil {
		t.Fatal(err)
	}
	if got, err := c.ListNetworks(ctx, map[string]string{"bakery.test": "aliases", "bakery.managed": "true"}); err != nil || len(got) != 1 || got[0] != network {
		t.Fatalf("ListNetworks: %v %v", got, err)
	}
	// Only when missing: Docker Hub now and then refuses a token request.
	if ok, _ := c.ImageExists(ctx, "docker.io/library/busybox"); !ok {
		if err := c.PullImage(ctx, "docker.io/library/busybox", func(string) {}); err != nil {
			t.Fatal(err)
		}
	}
	for i, n := range names {
		net, opts := NetworkWithAliases(network, []string{"db", "app"}[i])
		id, err := c.CreateContainer(ctx, ContainerSpec{
			Name: n, Image: "docker.io/library/busybox",
			Entrypoint: []string{"sh", "-c"}, Command: []string{"sleep 300"},
			WorkDir: "/tmp", User: "65534", Labels: labels,
			Networks: map[string]map[string]any{net: opts},
		})
		if err != nil {
			t.Fatalf("CreateContainer %s: %v", n, err)
		}
		if err := c.StartContainer(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	// busybox's nslookup trips over Podman's DNS server; ping prints the
	// address it resolved even where rootless ping itself is not allowed.
	code, out, err := c.Exec(ctx, names[1], []string{"sh", "-c", "pwd; id -u; ping -c1 -W1 db 2>&1 | head -1"})
	if err != nil || code != 0 {
		t.Fatalf("exec: %d %v\n%s", code, err, out)
	}
	if !strings.HasPrefix(out, "/tmp\n65534\n") || !strings.Contains(out, "PING db (") {
		t.Errorf("exec output:\n%s", out)
	}
	for _, n := range names {
		if err := c.RemoveContainer(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.RemoveNetwork(ctx, network); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveNetwork(ctx, network); err != nil {
		t.Errorf("removing a missing network: %v", err)
	}
}

func TestBuildNoCache(t *testing.T) {
	c := client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	busybox(t, ctx, c)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM docker.io/library/busybox:latest\nRUN echo layer > /layer\n"), 0o644)
	tag := "localhost/bakery-test/nocache:1"
	defer c.RemoveImage(context.Background(), tag)
	build := func(noCache bool) string {
		var out []string
		if _, err := c.Build(ctx, TarDir(dir), BuildOptions{Tag: tag, Dockerfile: "Containerfile", NoCache: noCache}, func(l string) { out = append(out, l) }); err != nil {
			t.Fatalf("build: %v", err)
		}
		return strings.Join(out, "\n")
	}
	build(false)
	if log := build(false); !strings.Contains(log, "Using cache") {
		t.Fatalf("second build did not use the cache:\n%s", log)
	}
	if log := build(true); strings.Contains(log, "Using cache") {
		t.Fatalf("no-cache build used the cache:\n%s", log)
	}
}
