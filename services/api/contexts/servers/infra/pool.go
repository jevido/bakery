package infra

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// Reach is how another context reaches one Server: its Podman API and any
// unix socket on it.
type Reach struct {
	Podman   *podman.Client
	DialUnix func(ctx context.Context, path string) (net.Conn, error)
}

// aliveTimeout bounds the keepalive that tells a pooled connection is still
// usable.
const aliveTimeout = 5 * time.Second

// Pool keeps one SSH connection per Remote server for other contexts, so a
// Deployment's many Podman calls do not each pay an SSH handshake.
type Pool struct {
	// Local is the client for the Local server's socket.
	Local *podman.Client

	mu    sync.Mutex
	conns map[uint64]*pooled
}

// pooled is one Server's slot; its own lock means dialling an unreachable
// Server never holds up reaching another.
type pooled struct {
	mu     sync.Mutex
	ssh    *podman.SSHConn
	client *podman.Client
}

func (p *Pool) slot(id uint64) *pooled {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conns == nil {
		p.conns = map[uint64]*pooled{}
	}
	c, ok := p.conns[id]
	if !ok {
		c = &pooled{}
		p.conns[id] = c
	}
	return c
}

// Reach returns the pooled connection to s, dialling a new one when there is
// none or the last one stopped answering.
func (p *Pool) Reach(ctx context.Context, s domain.Server) (Reach, error) {
	if s.Kind == domain.Local {
		return Reach{Podman: p.Local, DialUnix: func(ctx context.Context, path string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}}, nil
	}
	c := p.slot(s.ID)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ssh != nil {
		if c.ssh.Alive(aliveTimeout) {
			return c.reach(), nil
		}
		c.ssh.Close()
		c.ssh, c.client = nil, nil
	}
	conn, err := podman.DialSSH(ctx, podman.SSHTarget{
		Host: s.Host, Port: s.Port, User: s.User,
		PrivateKey: []byte(s.Key.Private), HostKey: s.HostKey,
	})
	var mismatch *podman.HostKeyMismatchError
	if errors.As(err, &mismatch) {
		return Reach{}, &app.HostKeyChangedError{Detail: mismatch.Error()}
	}
	if err != nil {
		return Reach{}, err
	}
	c.ssh, c.client = conn, conn.Podman()
	return c.reach(), nil
}

func (c *pooled) reach() Reach {
	return Reach{Podman: c.client, DialUnix: c.ssh.DialUnix}
}

// Forget closes and drops the Server's pooled connection, if any.
func (p *Pool) Forget(id uint64) {
	p.mu.Lock()
	c, ok := p.conns[id]
	delete(p.conns, id)
	p.mu.Unlock()
	if ok {
		c.mu.Lock()
		if c.ssh != nil {
			c.ssh.Close()
		}
		c.mu.Unlock()
	}
}
