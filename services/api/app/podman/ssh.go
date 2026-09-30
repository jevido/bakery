package podman

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHTarget is a remote user whose rootless Podman is reached over SSH.
type SSHTarget struct {
	Host string
	Port int
	User string
	// PrivateKey is the OpenSSH PEM of the key Bakery logs in with.
	PrivateKey []byte
	// HostKey is the pinned host key as an authorized_keys line; empty
	// accepts whatever the server presents (first use), which
	// SSHConn.HostKey then reports so the caller can pin it.
	HostKey string
	// Socket is the remote Podman socket; empty means the user's rootless
	// socket, /run/user/<uid>/podman/podman.sock.
	Socket string
}

// HostKeyMismatchError is returned when the server presents another host key
// than the pinned one.
type HostKeyMismatchError struct {
	Want, Got string
}

func (e *HostKeyMismatchError) Error() string {
	return "ssh: host key changed (pinned " + fingerprint(e.Want) + ", server presented " + fingerprint(e.Got) + ")"
}

func fingerprint(authorizedKey string) string {
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return "?"
	}
	return ssh.FingerprintSHA256(k)
}

// dialTimeout bounds the TCP connect and the SSH handshake together.
const dialTimeout = 10 * time.Second

// SSHConn is one SSH connection to a Server. Close it when done.
type SSHConn struct {
	client  *ssh.Client
	target  SSHTarget
	hostKey string

	once   sync.Once
	socket string
	err    error
}

// DialSSH connects and authenticates with the target's key, checking the
// host key against the pinned one.
func DialSSH(ctx context.Context, t SSHTarget) (*SSHConn, error) {
	signer, err := ssh.ParsePrivateKey(t.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("ssh: private key: %w", err)
	}
	port := t.Port
	if port == 0 {
		port = 22
	}
	conn := &SSHConn{target: t}
	cfg := &ssh.ClientConfig{
		User:    t.User,
		Auth:    []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout: dialTimeout,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			got := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			conn.hostKey = got
			if t.HostKey == "" {
				return nil
			}
			want, _, _, _, err := ssh.ParseAuthorizedKey([]byte(t.HostKey))
			if err != nil || !bytes.Equal(want.Marshal(), key.Marshal()) {
				return &HostKeyMismatchError{Want: t.HostKey, Got: got}
			}
			return nil
		},
	}
	addr := net.JoinHostPort(t.Host, strconv.Itoa(port))
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	tcp, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh: %w", err)
	}
	// NewClientConn has no context; a deadline bounds the handshake instead.
	deadline, _ := ctx.Deadline()
	_ = tcp.SetDeadline(deadline)
	c, chans, reqs, err := ssh.NewClientConn(tcp, addr, cfg)
	if err != nil {
		tcp.Close()
		var mismatch *HostKeyMismatchError
		if errors.As(err, &mismatch) {
			return nil, mismatch
		}
		return nil, fmt.Errorf("ssh: %w", err)
	}
	_ = tcp.SetDeadline(time.Time{})
	conn.client = ssh.NewClient(c, chans, reqs)
	return conn, nil
}

// HostKey is the host key the server presented, as an authorized_keys line.
func (c *SSHConn) HostKey() string { return c.hostKey }

// Run runs a plain command (never the podman CLI) and returns its stdout.
// A non-zero exit is an error carrying stderr.
func (c *SSHConn) Run(ctx context.Context, cmd string) (string, error) {
	s, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh: %w", err)
	}
	defer s.Close()
	var stdout, stderr bytes.Buffer
	s.Stdout, s.Stderr = &stdout, &stderr
	done := make(chan error, 1)
	go func() { done <- s.Run(cmd) }()
	select {
	case <-ctx.Done():
		_ = s.Signal(ssh.SIGKILL)
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				return stdout.String(), fmt.Errorf("ssh: %s: %w", cmd, err)
			}
			return stdout.String(), fmt.Errorf("ssh: %s: %w: %s", cmd, err, msg)
		}
		return stdout.String(), nil
	}
}

// Socket is the remote Podman socket: the target's, or the user's rootless
// one found through `id -u`.
func (c *SSHConn) Socket(ctx context.Context) (string, error) {
	c.once.Do(func() {
		if c.target.Socket != "" {
			c.socket = c.target.Socket
			return
		}
		out, err := c.Run(ctx, "id -u")
		if err != nil {
			c.err = err
			return
		}
		uid := strings.TrimSpace(out)
		if _, err := strconv.Atoi(uid); err != nil {
			c.err = fmt.Errorf("ssh: id -u printed %q", uid)
			return
		}
		c.socket = "/run/user/" + uid + "/podman/podman.sock"
	})
	return c.socket, c.err
}

// Podman is a client for the remote socket, each API connection a
// direct-streamlocal channel over this SSH connection.
func (c *SSHConn) Podman() *Client {
	return NewDialer(func(ctx context.Context) (net.Conn, error) {
		sock, err := c.Socket(ctx)
		if err != nil {
			return nil, err
		}
		conn, err := c.client.DialContext(ctx, "unix", sock)
		if err != nil {
			return nil, fmt.Errorf("ssh: %s: %w", sock, err)
		}
		return conn, nil
	})
}

// Close ends the SSH connection and every channel on it.
func (c *SSHConn) Close() error { return c.client.Close() }
