package podman

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// sshStandIn is an in-process SSH server that accepts one client key, runs
// `id -u` (printing uid) and forwards direct-streamlocal channels to local
// unix sockets, as OpenSSH's sshd does.
type sshStandIn struct {
	addr    string
	hostKey string // authorized_keys line
	uid     string
}

func newKey(t *testing.T) (ssh.Signer, []byte) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	return signer, pem.EncodeToMemory(block)
}

func startSSHStandIn(t *testing.T, client ssh.PublicKey) *sshStandIn {
	t.Helper()
	host, _ := newKey(t)
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		if string(k.Marshal()) != string(client.Marshal()) {
			return nil, errors.New("unknown key")
		}
		return nil, nil
	}}
	cfg.AddHostKey(host)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	s := &sshStandIn{addr: l.Addr().String(), hostKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(host.PublicKey()))), uid: "1000"}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go s.serve(c, cfg)
		}
	}()
	return s
}

func (s *sshStandIn) serve(c net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		switch nc.ChannelType() {
		case "direct-streamlocal@openssh.com":
			var msg struct {
				Path      string
				Reserved0 string
				Reserved1 uint32
			}
			if err := ssh.Unmarshal(nc.ExtraData(), &msg); err != nil {
				nc.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			local, err := net.Dial("unix", msg.Path)
			if err != nil {
				nc.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			ch, creqs, _ := nc.Accept()
			go ssh.DiscardRequests(creqs)
			go func() { io.Copy(ch, local); ch.CloseWrite(); ch.Close() }()
			go func() { io.Copy(local, ch); local.Close() }()
		case "session":
			ch, creqs, _ := nc.Accept()
			go func() {
				for r := range creqs {
					if r.Type != "exec" {
						r.Reply(false, nil)
						continue
					}
					var cmd struct{ Command string }
					ssh.Unmarshal(r.Payload, &cmd)
					r.Reply(true, nil)
					status := uint32(0)
					if cmd.Command == "id -u" {
						io.WriteString(ch, s.uid+"\n")
					} else {
						io.WriteString(ch.Stderr(), "nope\n")
						status = 1
					}
					ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
					ch.Close()
				}
			}()
		default:
			nc.Reject(ssh.UnknownChannelType, "no")
		}
	}
}

// podmanPingSocket serves _ping on a unix socket, like the Podman service.
func podmanPingSocket(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "podman.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == apiPrefix+"/_ping" {
			io.WriteString(w, "OK")
			return
		}
		http.NotFound(w, r)
	})}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return sock
}

func target(t *testing.T, s *sshStandIn, key []byte, hostKey, socket string) SSHTarget {
	host, port, _ := net.SplitHostPort(s.addr)
	p, _ := strconv.Atoi(port)
	return SSHTarget{Host: host, Port: p, User: "podman", PrivateKey: key, HostKey: hostKey, Socket: socket}
}

func TestSSHPingThroughTunnel(t *testing.T) {
	signer, key := newKey(t)
	s := startSSHStandIn(t, signer.PublicKey())
	sock := podmanPingSocket(t)
	ctx := context.Background()

	conn, err := DialSSH(ctx, target(t, s, key, "", sock))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn.HostKey() != s.hostKey {
		t.Fatalf("host key %q, want %q", conn.HostKey(), s.hostKey)
	}
	if err := conn.Podman().Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Pinned and matching.
	conn2, err := DialSSH(ctx, target(t, s, key, s.hostKey, sock))
	if err != nil {
		t.Fatal(err)
	}
	conn2.Close()
}

func TestSSHSocketFromUID(t *testing.T) {
	signer, key := newKey(t)
	s := startSSHStandIn(t, signer.PublicKey())
	conn, err := DialSSH(context.Background(), target(t, s, key, "", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sock, err := conn.Socket(context.Background())
	if err != nil || sock != "/run/user/1000/podman/podman.sock" {
		t.Fatalf("socket %q, %v", sock, err)
	}
	if _, err := conn.Run(context.Background(), "false"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("failing command: %v", err)
	}
}

func TestSSHHostKeyMismatch(t *testing.T) {
	signer, key := newKey(t)
	s := startSSHStandIn(t, signer.PublicKey())
	other, _ := newKey(t)
	pinned := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(other.PublicKey())))
	_, err := DialSSH(context.Background(), target(t, s, key, pinned, ""))
	var mismatch *HostKeyMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err %v, want HostKeyMismatchError", err)
	}
	if mismatch.Got != s.hostKey {
		t.Fatalf("got %q", mismatch.Got)
	}
}

func TestSSHWrongClientKey(t *testing.T) {
	signer, _ := newKey(t)
	s := startSSHStandIn(t, signer.PublicKey())
	_, other := newKey(t)
	if _, err := DialSSH(context.Background(), target(t, s, other, "", "")); err == nil {
		t.Fatal("dial with an unknown key succeeded")
	}
}
