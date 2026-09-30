//go:build podman

package infra

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// Runs a Remote Proxy on the Remote server stand-in (task remote:up), whose
// 80 is published on 127.0.0.1:4974, and checks a Route reaches a whoami
// container there, with the admin API only on its socket over SSH.
func TestRemoteProxyOnStandIn(t *testing.T) {
	if c, err := net.DialTimeout("tcp", "127.0.0.1:4972", time.Second); err != nil {
		t.Skip("remote stand-in not running (task remote:up)")
	} else {
		c.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	local := podman.New(podman.DefaultSocket())

	// Authorise a throwaway key for the stand-in's podman user.
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
	// Cleanups run last-registered first, so the connection outlives them.
	t.Cleanup(func() { conn.Close() })
	pc := conn.Podman()
	pc.RemoveContainer(ctx, "bakery-test-whoami") // left by an aborted run

	proxy := NewRemoteProxy(pc, conn.DialUnix, ProxyConfig{
		Name: "bakery-test-proxy", Image: "docker.io/library/caddy:2", Network: "bakery-test",
		InternalTLS: true, VolumePrefix: "bakery-test-proxy",
	})
	t.Cleanup(func() {
		bg := context.Background()
		pc.RemoveContainer(bg, "bakery-test-whoami")
		pc.RemoveContainer(bg, "bakery-test-proxy")
		for _, v := range []string{"data", "config", "admin"} {
			pc.RemoveVolume(bg, "bakery-test-proxy-"+v)
		}
	})
	if err := proxy.Ensure(ctx); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	info, err := pc.InspectContainer(ctx, "bakery-test-proxy")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("remote proxy %s running=%v", info.ID, info.State.Running)

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
	if err := pc.StartContainer(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := proxy.Apply(ctx, []domain.Route{{ApplicationID: 1, Domains: []string{"remote.localhost"}, Container: "bakery-test-whoami", Port: 80}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	waitFor(t, func() bool {
		body, code := plainGet("127.0.0.1:4974", "remote.localhost")
		return code == 200 && strings.Contains(body, "Hostname:")
	}, "whoami answering through the Remote Proxy")

	// A restarted Proxy comes back with its config and its admin socket.
	if err := pc.StopContainer(ctx, "bakery-test-proxy", 5); err != nil {
		t.Fatal(err)
	}
	fresh := NewRemoteProxy(pc, conn.DialUnix, ProxyConfig{
		Name: "bakery-test-proxy", Image: "docker.io/library/caddy:2", Network: "bakery-test",
		InternalTLS: true, VolumePrefix: "bakery-test-proxy",
	})
	if err := fresh.Ensure(ctx); err != nil {
		t.Fatalf("Ensure after stop: %v", err)
	}
	if err := fresh.Apply(ctx, nil); err != nil {
		t.Fatalf("Apply after restart: %v", err)
	}
	waitFor(t, func() bool {
		// Caddy answers an unmatched host with an empty 200.
		body, _ := plainGet("127.0.0.1:4974", "remote.localhost")
		return !strings.Contains(body, "Hostname:")
	}, "the route gone after an empty Apply")
}

// plainGet requests http://host/ at addr.
func plainGet(addr, host string) (string, int) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/", nil)
	req.Host = host
	res, err := client.Do(req)
	if err != nil {
		return "", 0
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return string(body), res.StatusCode
}
