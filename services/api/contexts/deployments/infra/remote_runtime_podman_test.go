//go:build podman

package infra

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
)

// Builds, starts, probes and removes an Application Container on the Remote
// server stand-in (task remote:up) over SSH: the build context is streamed
// from here, as a Deployment's clone is.
func TestRuntimeOnStandIn(t *testing.T) {
	if c, err := net.DialTimeout("tcp", "127.0.0.1:4972", time.Second); err != nil {
		t.Skip("remote stand-in not running (task remote:up)")
	} else {
		c.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	local := podman.New(podman.DefaultSocket())
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	block, _ := ssh.MarshalPrivateKey(priv, "")
	public := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if code, out, err := local.Exec(ctx, "bakery-dev-remote-1", []string{"sh", "-c", "echo '" + public + "' >> /home/podman/.ssh/authorized_keys"}); err != nil || code != 0 {
		t.Fatalf("authorising: %d %s %v", code, out, err)
	}
	conn, err := podman.DialSSH(ctx, podman.SSHTarget{Host: "127.0.0.1", Port: 4972, User: "podman", PrivateKey: pem.EncodeToMemory(block)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	c := conn.Podman()
	if err := c.EnsureNetwork(ctx, "bakery-test"); err != nil {
		t.Fatal(err)
	}
	r := Runtime{Server: "stand-in", Podman: c, Network: "bakery-test", StartTimeout: 30 * time.Second, Settle: time.Second}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM docker.io/library/busybox:latest\nRUN mkdir /www && echo remote > /www/health\n"+
		`CMD ["httpd", "-f", "-p", "8080", "-h", "/www"]`+"\n"), 0o644)
	const tag = "localhost/bakery-test/remote:1"
	t.Cleanup(func() { c.RemoveImage(context.Background(), tag) })
	if err := r.Build(ctx, app.BuildRequest{Dir: dir, Dockerfile: "Containerfile", Tag: tag, Labels: map[string]string{"bakery.managed": "true"}}, func(line string) { t.Log(line) }); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if ok, err := r.ImageExists(ctx, tag); err != nil || !ok {
		t.Fatalf("image on the stand-in: %v %v", ok, err)
	}
	if ok, _ := local.ImageExists(ctx, tag); ok {
		t.Fatal("the image was built locally")
	}

	const appID = 990011
	t.Cleanup(func() {
		r.RemoveAll(context.Background(), appID)
		r.RemoveVolumes(context.Background(), appID)
	})
	spec := app.ContainerSpec{Name: "bakery-test-remote-1", Image: tag, ApplicationID: appID, DeploymentID: 1,
		Mounts: []app.Mount{{Volume: "bakery-app-990011-data", Path: "/data"}}}
	if err := r.Start(ctx, spec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	var ok bool
	var detail string
	for range 20 {
		if ok, detail, err = r.Probe(ctx, spec.Name, "http://127.0.0.1:8080/health", 2*time.Second); ok || err != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !ok || err != nil {
		t.Fatalf("probe: %v %s %v", ok, detail, err)
	}
	if name, found, err := r.Running(ctx, appID); err != nil || !found || name != spec.Name {
		t.Fatalf("running: %q %v %v", name, found, err)
	}
	if err := r.RemoveAll(ctx, appID); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveVolumes(ctx, appID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := r.Running(ctx, appID); found {
		t.Fatal("still running after RemoveAll")
	}
}
