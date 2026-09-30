// Package app holds the notifications use cases: manage Notification
// channels, test them, and deliver Notifications to them.
package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// ErrNotFound is a Notification channel that does not exist.
var ErrNotFound = errors.New("not found")

// Store keeps Notification channels.
type Store interface {
	Channels(ctx context.Context) ([]domain.Channel, error)
	Channel(ctx context.Context, id uint64) (domain.Channel, bool, error)
	CreateChannel(ctx context.Context, c domain.Channel) (domain.Channel, error)
	SaveChannel(ctx context.Context, c domain.Channel) error
	DeleteChannel(ctx context.Context, id uint64) error
	ChannelNameTaken(ctx context.Context, name string, exceptID uint64) (bool, error)
}

type Service struct {
	store Store
	Log   func(format string, args ...any)
}

func NewService(store Store) *Service {
	return &Service{store: store, Log: func(string, ...any) {}}
}

func (s *Service) Channels(ctx context.Context) ([]domain.Channel, error) {
	return s.store.Channels(ctx)
}

func (s *Service) Channel(ctx context.Context, id uint64) (domain.Channel, error) {
	c, found, err := s.store.Channel(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return c, err
}

func (s *Service) checkName(ctx context.Context, c domain.Channel) error {
	taken, err := s.store.ChannelNameTaken(ctx, c.Name, c.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "name", Message: "another channel has this name"}
	}
	return nil
}

func (s *Service) AddChannel(ctx context.Context, in domain.Input) (domain.Channel, error) {
	c, err := domain.NewChannel(in)
	if err != nil {
		return c, err
	}
	if err := s.checkName(ctx, c); err != nil {
		return c, err
	}
	return s.store.CreateChannel(ctx, c)
}

// ChangeChannel changes the channel; empty secret settings keep theirs.
func (s *Service) ChangeChannel(ctx context.Context, id uint64, in domain.Input) (domain.Channel, error) {
	c, err := s.Channel(ctx, id)
	if err != nil {
		return c, err
	}
	if err := c.Change(in); err != nil {
		return c, err
	}
	if err := s.checkName(ctx, c); err != nil {
		return c, err
	}
	return c, s.store.SaveChannel(ctx, c)
}

func (s *Service) DeleteChannel(ctx context.Context, id uint64) error {
	if _, err := s.Channel(ctx, id); err != nil {
		return err
	}
	return s.store.DeleteChannel(ctx, id)
}
