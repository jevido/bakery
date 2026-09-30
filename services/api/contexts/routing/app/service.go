// Package app holds the routing use cases: keep the Proxy running, switch
// and drop Routes, and Apply the whole set to the Proxy after each change.
package app

import (
	"context"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// Routes stores Routes, one per Application.
type Routes interface {
	All(ctx context.Context) ([]domain.Route, error)
	Upsert(ctx context.Context, r domain.Route) error
	// ChangeDomains changes the Domains of an existing Route; found is
	// false when the Application has none.
	ChangeDomains(ctx context.Context, applicationID uint64, domains []string) (found bool, err error)
	Delete(ctx context.Context, applicationID uint64) error
}

// ServiceRoutes stores Service routes, replaced per Service as a set.
type ServiceRoutes interface {
	All(ctx context.Context) ([]domain.ServiceRoute, error)
	Replace(ctx context.Context, serviceID uint64, routes []domain.ServiceRoute) error
	Delete(ctx context.Context, serviceID uint64) error
}

// Settings stores Route settings, one per Application.
type Settings interface {
	All(ctx context.Context) (map[uint64]domain.RouteSettings, error)
	Get(ctx context.Context, applicationID uint64) (s domain.RouteSettings, found bool, err error)
	Put(ctx context.Context, s domain.RouteSettings) error
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
	routes        Routes
	serviceRoutes ServiceRoutes
	settings      Settings
	proxy         Proxy
	// mu makes read-all-then-Apply one step, so two changes at once cannot
	// Apply out of order and leave the older set loaded.
	mu sync.Mutex
}

func NewService(routes Routes, serviceRoutes ServiceRoutes, settings Settings, proxy Proxy) *Service {
	return &Service{routes: routes, serviceRoutes: serviceRoutes, settings: settings, proxy: proxy}
}

// RouteSettings returns the Application's Route settings, the defaults when
// none are stored.
func (s *Service) RouteSettings(ctx context.Context, applicationID uint64) (domain.RouteSettings, error) {
	rs, found, err := s.settings.Get(ctx, applicationID)
	if err != nil || !found {
		return domain.DefaultRouteSettings(applicationID), err
	}
	return rs, nil
}

// ChangeRouteSettings checks and stores the settings, then Applies. A
// non-empty password becomes the Basic auth password (only its bcrypt hash
// is kept); an empty one keeps the stored hash.
func (s *Service) ChangeRouteSettings(ctx context.Context, rs domain.RouteSettings, password string) (domain.RouteSettings, error) {
	if password == "" {
		current, err := s.RouteSettings(ctx, rs.ApplicationID)
		if err != nil {
			return rs, err
		}
		rs.BasicAuth.PasswordHash = current.BasicAuth.PasswordHash
	} else {
		// bcrypt only reads the first 72 bytes.
		if len(password) > 72 {
			return rs, &domain.FieldError{Field: "basic_auth.password", Message: "password is at most 72 bytes"}
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return rs, err
		}
		rs.BasicAuth.PasswordHash = string(hash)
	}
	rs, err := rs.Check()
	if err != nil {
		return rs, err
	}
	if err := s.settings.Put(ctx, rs); err != nil {
		return rs, err
	}
	return rs, s.apply(ctx)
}

// EnsureProxy makes sure the Proxy runs and serves every stored Route.
func (s *Service) EnsureProxy(ctx context.Context) error {
	if err := s.proxy.Ensure(ctx); err != nil {
		return err
	}
	return s.apply(ctx)
}

// ChangeDomains moves the Application's Route, if it has one, to the
// Domains, then Applies. Without a Route there is nothing to serve yet: the
// first Deployment passes the Domains.
func (s *Service) ChangeDomains(ctx context.Context, applicationID uint64, domains []string) error {
	found, err := s.routes.ChangeDomains(ctx, applicationID, domains)
	if err != nil || !found {
		return err
	}
	return s.apply(ctx)
}

// SwitchRoute points the Application's Domains at a new Container, then
// Applies.
func (s *Service) SwitchRoute(ctx context.Context, r domain.Route) error {
	if err := s.routes.Upsert(ctx, r); err != nil {
		return err
	}
	return s.apply(ctx)
}

// DropRoute removes the Application's Route and Route settings, then
// Applies.
func (s *Service) DropRoute(ctx context.Context, applicationID uint64) error {
	if err := s.routes.Delete(ctx, applicationID); err != nil {
		return err
	}
	if err := s.settings.Delete(ctx, applicationID); err != nil {
		return err
	}
	return s.apply(ctx)
}

// SetServiceRoutes makes routes the Service's whole set of Service routes,
// then Applies.
func (s *Service) SetServiceRoutes(ctx context.Context, serviceID uint64, routes []domain.ServiceRoute) error {
	if err := s.serviceRoutes.Replace(ctx, serviceID, routes); err != nil {
		return err
	}
	return s.apply(ctx)
}

// DropServiceRoutes removes the Service's Service routes, then Applies.
func (s *Service) DropServiceRoutes(ctx context.Context, serviceID uint64) error {
	if err := s.serviceRoutes.Delete(ctx, serviceID); err != nil {
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
	settings, err := s.settings.All(ctx)
	if err != nil {
		return err
	}
	for i, r := range all {
		if rs, ok := settings[r.ApplicationID]; ok {
			all[i].Settings = rs
		} else {
			all[i].Settings = domain.DefaultRouteSettings(r.ApplicationID)
		}
	}
	// Service routes carry their own default settings; they are appended
	// after the lookup above so no Application's settings reach them.
	serviceRoutes, err := s.serviceRoutes.All(ctx)
	if err != nil {
		return err
	}
	for _, r := range serviceRoutes {
		all = append(all, r.Route())
	}
	return s.proxy.Apply(ctx, all)
}
