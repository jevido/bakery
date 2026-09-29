// Package app holds the routing use cases: keep the Proxy running, switch
// and drop Routes, and Apply the whole set to the Proxy after each change.
package app

import (
	"context"
	"sync"

	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// Routes stores Routes, one per Application.
type Routes interface {
	All(ctx context.Context) ([]domain.Route, error)
	Upsert(ctx context.Context, r domain.Route) error
	Delete(ctx context.Context, applicationID uint64) error
}

// Proxy is the Caddy container.
type Proxy interface {
	// Ensure creates or starts the Proxy container.
	Ensure(ctx context.Context) error
	// Apply replaces the Proxy's whole configuration with these Routes.
	Apply(ctx context.Context, routes []domain.Route) error
}

type Service struct {
	routes Routes
	proxy  Proxy
	// mu makes read-all-then-Apply one step, so two changes at once cannot
	// Apply out of order and leave the older set loaded.
	mu sync.Mutex
}

func NewService(routes Routes, proxy Proxy) *Service {
	return &Service{routes: routes, proxy: proxy}
}

// EnsureProxy makes sure the Proxy runs and serves every stored Route.
func (s *Service) EnsureProxy(ctx context.Context) error {
	if err := s.proxy.Ensure(ctx); err != nil {
		return err
	}
	return s.apply(ctx)
}

// SwitchRoute points the Application's Domain at a new Container, then
// Applies.
func (s *Service) SwitchRoute(ctx context.Context, r domain.Route) error {
	if err := s.routes.Upsert(ctx, r); err != nil {
		return err
	}
	return s.apply(ctx)
}

// DropRoute removes the Application's Route, then Applies.
func (s *Service) DropRoute(ctx context.Context, applicationID uint64) error {
	if err := s.routes.Delete(ctx, applicationID); err != nil {
		return err
	}
	return s.apply(ctx)
}

func (s *Service) apply(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.routes.All(ctx)
	if err != nil {
		return err
	}
	return s.proxy.Apply(ctx, all)
}
