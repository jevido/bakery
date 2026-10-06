//go:build podman

package infra

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type keyPair struct{ Public, Private string }

// testKey makes an ed25519 key pair the way a Deploy key is made.
func testKey() (keyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return keyPair{}, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "bakery-test")
	if err != nil {
		return keyPair{}, err
	}
	sshPub, _ := ssh.NewPublicKey(pub)
	return keyPair{Public: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))), Private: string(pem.EncodeToMemory(block))}, nil
}

// memKnownHosts is KnownHosts without a database.
type memKnownHosts struct {
	mu    sync.Mutex
	lines string
}

func (m *memKnownHosts) Lines(context.Context, uint64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lines, nil
}
func (m *memKnownHosts) Remember(_ context.Context, _ uint64, lines string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = lines
	return nil
}
func (m *memKnownHosts) List(context.Context, uint64) ([]domain.KnownHost, error) { return nil, nil }
func (m *memKnownHosts) Forget(context.Context, uint64, uint64) (bool, error)     { return false, nil }

// Runs a throwaway sshd + git container (bakery-test-sshgit on 127.0.0.1:4958)
// and clones a repository from it with a Deploy key: first use trusts the
// host key, a wrong key is refused, a changed host key fails the clone.
func TestCloneOverSSH(t *testing.T) {
	pc := podman.New(podman.DefaultSocket())
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := pc.Ping(ctx); err != nil {
		t.Skipf("no podman: %v", err)
	}
	key, err := testKey()
	if err != nil {
		t.Fatal(err)
	}
	image := "docker.io/library/alpine:3"
	if ok, _ := pc.ImageExists(ctx, image); !ok {
		if err := pc.PullImage(ctx, image, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := pc.EnsureNetwork(ctx, "bakery-test"); err != nil {
		t.Fatal(err)
	}
	script := `set -e
apk add --no-cache openssh git >/dev/null
ssh-keygen -A >/dev/null
adduser -D -s /usr/bin/git-shell git && sed -i 's/^git:!/git:*/' /etc/shadow
mkdir -p /home/git/.ssh && echo "$AUTH_KEY" > /home/git/.ssh/authorized_keys
git init -q --bare -b main /home/git/app.git
git init -q -b main /tmp/w && cd /tmp/w && echo hi > README
git add README && git -c user.name=Jane -c user.email=j@x commit -qm "First commit"
git push -q /home/git/app.git main
chown -R git /home/git && chmod 700 /home/git/.ssh
touch /ready
exec /usr/sbin/sshd -D -e -p 22`
	id, err := pc.CreateContainer(ctx, podman.ContainerSpec{
		Name: "bakery-test-sshgit", Image: image, Command: []string{"sh", "-c", script},
		Env:          map[string]string{"AUTH_KEY": key.Public},
		Labels:       map[string]string{"bakery.test": "true"},
		Networks:     podman.OnNetwork("bakery-test"),
		PortMappings: []podman.PortMapping{{HostIP: "127.0.0.1", HostPort: 4958, ContainerPort: 22}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.RemoveContainer(context.Background(), id) })
	if err := pc.StartContainer(ctx, id); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := pc.ReadFile(ctx, id, "/ready"); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("sshd container never got ready")
		case <-time.After(time.Second):
		}
	}
	time.Sleep(time.Second) // sshd binding

	known := &memKnownHosts{}
	g := Git{KnownHosts: known}
	url := "ssh://git@127.0.0.1:4958/home/git/app.git"
	var log []string
	out := func(_, line string) { log = append(log, line) }
	clone := func(k string) (app.Commit, error) {
		log = nil
		return g.Clone(ctx, app.CloneRequest{GuildID: 1, URL: url, Branch: "main", Dir: filepath.Join(t.TempDir(), "c"), DeployKey: k}, out)
	}

	c, err := clone(key.Private)
	if err != nil {
		t.Fatalf("first clone: %v\n%s", err, strings.Join(log, "\n"))
	}
	if c.Subject != "First commit" || c.Author != "Jane" {
		t.Fatalf("commit: %+v", c)
	}
	if !strings.HasPrefix(known.lines, "[127.0.0.1]:4958 ") {
		t.Fatalf("host key not remembered: %q", known.lines)
	}
	if _, err := clone(key.Private); err != nil {
		t.Fatalf("second clone with the remembered key: %v", err)
	}

	other, _ := testKey()
	if _, err := clone(other.Private); err == nil || !strings.Contains(err.Error(), "refused the deploy key") {
		t.Fatalf("wrong deploy key: %v", err)
	}

	// Pretend the host's key changed: remember another ed25519 key for it.
	fake := strings.Fields(other.Public)
	known.lines = "[127.0.0.1]:4958 " + fake[0] + " " + fake[1] + "\n"
	if _, err := clone(key.Private); err == nil || !strings.Contains(err.Error(), "changed since the first clone") {
		t.Fatalf("changed host key: %v\n%s", err, strings.Join(log, "\n"))
	}
}
