// Package app holds the servers use cases.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

var (
	// ErrNotFound is returned for a Server that does not exist.
	ErrNotFound = errors.New("server not found")
	// ErrInUse is a Delete of a Server something still runs on.
	ErrInUse = errors.New("the server still runs applications; delete them first")
	// ErrNotValidated is a connection to a Remote server whose Host key was
	// never pinned: without a Validation there is no identity to check.
	ErrNotValidated = errors.New("the server has not been validated yet")
)

// InUse reports whether something still runs on the Server (named as
// RefID names it).
type InUse func(ctx context.Context, serverID uint64) (bool, error)

// Store keeps Servers.
type Store interface {
	Create(ctx context.Context, s domain.Server) (domain.Server, error)
	Get(ctx context.Context, id uint64) (domain.Server, bool, error)
	// List returns the Local server first, then the Guild's Remote servers
	// oldest first; guildID 0 lists every Server.
	List(ctx context.Context, guildID uint64) ([]domain.Server, error)
	Save(ctx context.Context, s domain.Server) error
	Delete(ctx context.Context, id uint64) error
	// NameTaken looks at the Guild's Servers and the Local server; guildID
	// 0 (the Local server's own name) at every Server.
	NameTaken(ctx context.Context, guildID uint64, name string, exceptID uint64) (bool, error)
	// AddressTaken looks at the Guild's Servers only: another Guild may
	// reach the same machine with its own key.
	AddressTaken(ctx context.Context, guildID uint64, host string, port int, user string, exceptID uint64) (bool, error)
	Local(ctx context.Context) (domain.Server, bool, error)
}

// Service runs the servers use cases.
type Service struct {
	store Store
	// newKey generates a Private key with the comment.
	newKey    func(comment string) (domain.PrivateKey, error)
	connector Connector
	now       func() time.Time
	// Retention applies Image retention on the Local server; nil skips it.
	Retention ImageRetention
	// Log reports failures that do not fail the use case.
	Log func(format string, args ...any)
	// Forget drops what is cached about reaching the Server (its pooled
	// connection) after it changed or went; nil does nothing.
	Forget func(id uint64)
	// OnHealthChanged, when set, hears what a Server probe found changed.
	OnHealthChanged func(ctx context.Context, e HealthChanged)
	inUse           []InUse
}

// OnDeleting registers a check asked before a Server is deleted.
func (s *Service) OnDeleting(check InUse) { s.inUse = append(s.inUse, check) }

func (s *Service) forget(id uint64) {
	if s.Forget != nil {
		s.Forget(id)
	}
}

// Reach returns the Server another context wants to run things on: 0 is the
// Local server. A Remote server must have a pinned Host key.
func (s *Service) Reach(ctx context.Context, id uint64) (domain.Server, error) {
	if id == 0 {
		return s.EnsureLocal(ctx)
	}
	srv, err := s.Get(ctx, id)
	if err != nil {
		return srv, err
	}
	if srv.Kind == domain.Remote && srv.HostKey == "" {
		return srv, ErrNotValidated
	}
	return srv, nil
}

func (s *Service) log(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

func NewService(store Store, newKey func(comment string) (domain.PrivateKey, error), connector Connector) *Service {
	return &Service{store: store, newKey: newKey, connector: connector, now: func() time.Time { return time.Now().UTC() }}
}

// EnsureLocal creates the Local server unless it exists, and returns it.
func (s *Service) EnsureLocal(ctx context.Context) (domain.Server, error) {
	srv, found, err := s.store.Local(ctx)
	if err != nil || found {
		return srv, err
	}
	return s.store.Create(ctx, domain.NewLocal())
}

// List returns the Servers the Guild may use: the Local server and its own.
func (s *Service) List(ctx context.Context, guildID uint64) ([]domain.Server, error) {
	return s.store.List(ctx, guildID)
}

func (s *Service) Get(ctx context.Context, id uint64) (domain.Server, error) {
	srv, found, err := s.store.Get(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return srv, err
}

func (s *Service) checkUnique(ctx context.Context, srv domain.Server) error {
	taken, err := s.store.NameTaken(ctx, srv.GuildID, srv.Name, srv.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "name", Message: "another server has this name"}
	}
	taken, err = s.store.AddressTaken(ctx, srv.GuildID, srv.Host, srv.Port, srv.User, srv.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "host", Message: "another server is reached as this user on this host and port"}
	}
	return nil
}

// Add creates a Remote server of the Guild with a new Private key.
func (s *Service) Add(ctx context.Context, guildID uint64, in domain.Input) (domain.Server, error) {
	// Validate the input before spending a key on it.
	if _, err := domain.NewRemote(guildID, in, domain.PrivateKey{Public: "-", Private: "-"}); err != nil {
		return domain.Server{}, err
	}
	key, err := s.newKey("bakery@" + in.Name)
	if err != nil {
		return domain.Server{}, err
	}
	srv, err := domain.NewRemote(guildID, in, key)
	if err != nil {
		return domain.Server{}, err
	}
	if err := s.checkUnique(ctx, srv); err != nil {
		return domain.Server{}, err
	}
	return s.store.Create(ctx, srv)
}

// Edit changes a Remote server.
func (s *Service) Edit(ctx context.Context, id uint64, in domain.Input) (domain.Server, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return srv, err
	}
	if err := srv.Edit(in); err != nil {
		return srv, err
	}
	if err := s.checkUnique(ctx, srv); err != nil {
		return srv, err
	}
	if err := s.store.Save(ctx, srv); err != nil {
		return srv, err
	}
	s.forget(id)
	return srv, nil
}

// Delete removes a Remote server.
func (s *Service) Delete(ctx context.Context, id uint64) error {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := srv.CanDelete(); err != nil {
		return err
	}
	for _, inUse := range s.inUse {
		used, err := inUse(ctx, srv.RefID())
		if err != nil {
			return err
		}
		if used {
			return ErrInUse
		}
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.forget(id)
	return nil
}

// ForgetHostKey drops a Remote server's pinned Host key.
func (s *Service) ForgetHostKey(ctx context.Context, id uint64) (domain.Server, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return srv, err
	}
	if err := srv.ForgetHostKey(); err != nil {
		return srv, err
	}
	if err := s.store.Save(ctx, srv); err != nil {
		return srv, err
	}
	s.forget(id)
	return srv, nil
}
