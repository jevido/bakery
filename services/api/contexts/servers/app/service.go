// Package app holds the servers use cases.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

// ErrNotFound is returned for a Server that does not exist.
var ErrNotFound = errors.New("server not found")

// Store keeps Servers.
type Store interface {
	Create(ctx context.Context, s domain.Server) (domain.Server, error)
	Get(ctx context.Context, id uint64) (domain.Server, bool, error)
	// List returns the Local server first, then oldest first.
	List(ctx context.Context) ([]domain.Server, error)
	Save(ctx context.Context, s domain.Server) error
	Delete(ctx context.Context, id uint64) error
	NameTaken(ctx context.Context, name string, exceptID uint64) (bool, error)
	AddressTaken(ctx context.Context, host string, port int, user string, exceptID uint64) (bool, error)
	Local(ctx context.Context) (domain.Server, bool, error)
}

// Service runs the servers use cases.
type Service struct {
	store Store
	// newKey generates a Server key with the comment.
	newKey    func(comment string) (domain.ServerKey, error)
	connector Connector
	now       func() time.Time
	// Log reports failures that do not fail the use case.
	Log func(format string, args ...any)
}

func (s *Service) log(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

func NewService(store Store, newKey func(comment string) (domain.ServerKey, error), connector Connector) *Service {
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

func (s *Service) List(ctx context.Context) ([]domain.Server, error) {
	return s.store.List(ctx)
}

func (s *Service) Get(ctx context.Context, id uint64) (domain.Server, error) {
	srv, found, err := s.store.Get(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return srv, err
}

func (s *Service) checkUnique(ctx context.Context, srv domain.Server) error {
	taken, err := s.store.NameTaken(ctx, srv.Name, srv.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "name", Message: "another server has this name"}
	}
	taken, err = s.store.AddressTaken(ctx, srv.Host, srv.Port, srv.User, srv.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "host", Message: "another server is reached as this user on this host and port"}
	}
	return nil
}

// Add creates a Remote server with a new Server key.
func (s *Service) Add(ctx context.Context, in domain.Input) (domain.Server, error) {
	// Validate the input before spending a key on it.
	if _, err := domain.NewRemote(in, domain.ServerKey{Public: "-", Private: "-"}); err != nil {
		return domain.Server{}, err
	}
	key, err := s.newKey("bakery@" + in.Name)
	if err != nil {
		return domain.Server{}, err
	}
	srv, err := domain.NewRemote(in, key)
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
	return srv, s.store.Save(ctx, srv)
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
	return s.store.Delete(ctx, id)
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
	return srv, s.store.Save(ctx, srv)
}
