// Package app holds the routing use cases: keep the Proxies running, switch
// and drop Routes, and Apply each Server's whole set to its Proxy after each
// change.
package app

import (
	"context"
	"slices"
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

// Proxies finds the Proxy of a Server (0 is the Local server).
type Proxies interface {
	For(ctx context.Context, serverID uint64) (Proxy, error)
}

// Proxy is the Caddy container on one Server.
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
	proxies       Proxies
	// locks make read-all-then-Apply one step per Server, so two changes at
	// once cannot Apply out of order and leave the older set loaded, while
	// a slow or unreachable Server never holds up another.
	mu    sync.Mutex
	locks map[uint64]*sync.Mutex
}

func NewService(routes Routes, serviceRoutes ServiceRoutes, settings Settings, proxies Proxies) *Service {
	return &Service{routes: routes, serviceRoutes: serviceRoutes, settings: settings, proxies: proxies, locks: map[uint64]*sync.Mutex{}}
}

func (s *Service) lock(serverID uint64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[serverID]
	if !ok {
		l = &sync.Mutex{}
		s.locks[serverID] = l
	}
	return l
}

// serverOf returns the Server the Application's Route is on; found is false
// when it has no Route.
func (s *Service) serverOf(ctx context.Context, applicationID uint64) (uint64, bool, error) {
	all, err := s.routes.All(ctx)
	if err != nil {
		return 0, false, err
	}
	for _, r := range all {
		if r.ApplicationID == applicationID {
			return r.ServerID, true, nil
		}
	}
	return 0, false, nil
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
	server, found, err := s.serverOf(ctx, rs.ApplicationID)
	if err != nil || !found {
		return rs, err
	}
	return rs, s.apply(ctx, server)
}

// EnsureProxy makes sure the Local server's Proxy runs and serves every
// Route there.
func (s *Service) EnsureProxy(ctx context.Context) error {
	return s.ensure(ctx, 0)
}

// RemoteServers lists the Remote servers that have Routes, whose Proxies
// EnsureRemoteProxy keeps.
func (s *Service) RemoteServers(ctx context.Context) ([]uint64, error) {
	all, err := s.routes.All(ctx)
	if err != nil {
		return nil, err
	}
	var out []uint64
	for _, r := range all {
		if r.ServerID != 0 && !slices.Contains(out, r.ServerID) {
			out = append(out, r.ServerID)
		}
	}
	return out, nil
}

// EnsureRemoteProxy makes sure a Remote server's Proxy runs and serves its
// Routes.
func (s *Service) EnsureRemoteProxy(ctx context.Context, serverID uint64) error {
	return s.ensure(ctx, serverID)
}

func (s *Service) ensure(ctx context.Context, serverID uint64) error {
	p, err := s.proxies.For(ctx, serverID)
	if err != nil {
		return err
	}
	if err := p.Ensure(ctx); err != nil {
		return err
	}
	return s.apply(ctx, serverID)
}

// ChangeDomains moves the Application's Route, if it has one, to the
// Domains, then Applies. Without a Route there is nothing to serve yet: the
// first Deployment passes the Domains.
func (s *Service) ChangeDomains(ctx context.Context, applicationID uint64, domains []string) error {
	found, err := s.routes.ChangeDomains(ctx, applicationID, domains)
	if err != nil || !found {
		return err
	}
	server, found, err := s.serverOf(ctx, applicationID)
	if err != nil || !found {
		return err
	}
	return s.apply(ctx, server)
}

// SwitchRoute points the Application's Domains at a new Container on its
// Server, makes sure that Server's Proxy runs (a Server's first Route
// creates it), then Applies that Server.
func (s *Service) SwitchRoute(ctx context.Context, r domain.Route) error {
	before, had, err := s.serverOf(ctx, r.ApplicationID)
	if err != nil {
		return err
	}
	if r.ServerID != 0 {
		p, err := s.proxies.For(ctx, r.ServerID)
		if err != nil {
			return err
		}
		if err := p.Ensure(ctx); err != nil {
			return err
		}
	}
	if err := s.routes.Upsert(ctx, r); err != nil {
		return err
	}
	if err := s.apply(ctx, r.ServerID); err != nil {
		return err
	}
	if had && before != r.ServerID {
		// The Route left that Server; its Proxy must stop serving it.
		return s.apply(ctx, before)
	}
	return nil
}

// DropRoute removes the Application's Route and Route settings, then
// Applies.
func (s *Service) DropRoute(ctx context.Context, applicationID uint64) error {
	server, found, err := s.serverOf(ctx, applicationID)
	if err != nil {
		return err
	}
	if err := s.routes.Delete(ctx, applicationID); err != nil {
		return err
	}
	if err := s.settings.Delete(ctx, applicationID); err != nil {
		return err
	}
	if !found {
		return nil
	}
	return s.apply(ctx, server)
}

// SetServiceRoutes makes routes the Service's whole set of Service routes,
// then Applies.
func (s *Service) SetServiceRoutes(ctx context.Context, serviceID uint64, routes []domain.ServiceRoute) error {
	if err := s.serviceRoutes.Replace(ctx, serviceID, routes); err != nil {
		return err
	}
	return s.apply(ctx, 0)
}

// DropServiceRoutes removes the Service's Service routes, then Applies.
func (s *Service) DropServiceRoutes(ctx context.Context, serviceID uint64) error {
	if err := s.serviceRoutes.Delete(ctx, serviceID); err != nil {
		return err
	}
	return s.apply(ctx, 0)
}

// apply loads the Server's Routes into its Proxy; the Local server's also
// gets the Service routes (Services run there).
func (s *Service) apply(ctx context.Context, serverID uint64) error {
	l := s.lock(serverID)
	l.Lock()
	defer l.Unlock()
	p, err := s.proxies.For(ctx, serverID)
	if err != nil {
		return err
	}
	stored, err := s.routes.All(ctx)
	if err != nil {
		return err
	}
	var all []domain.Route
	for _, r := range stored {
		if r.ServerID == serverID {
			all = append(all, r)
		}
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
	if serverID != 0 {
		return p.Apply(ctx, all)
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
	return p.Apply(ctx, all)
}
