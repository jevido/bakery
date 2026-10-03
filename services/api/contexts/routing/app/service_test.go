package app

import (
	"context"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

type fakeRoutes struct{ routes []domain.Route }

func (f *fakeRoutes) All(context.Context) ([]domain.Route, error) { return slices.Clone(f.routes), nil }
func (f *fakeRoutes) Upsert(_ context.Context, r domain.Route) error {
	f.routes = append(f.routes, r)
	return nil
}
func (f *fakeRoutes) ChangeDomains(context.Context, uint64, []string) (bool, error) {
	return false, nil
}
func (f *fakeRoutes) Delete(context.Context, uint64) error { return nil }

type fakeServiceRoutes struct{ routes []domain.ServiceRoute }

func (f *fakeServiceRoutes) All(context.Context) ([]domain.ServiceRoute, error) {
	return slices.Clone(f.routes), nil
}
func (f *fakeServiceRoutes) Replace(ctx context.Context, serviceID uint64, routes []domain.ServiceRoute) error {
	f.Delete(ctx, serviceID)
	f.routes = append(f.routes, routes...)
	return nil
}
func (f *fakeServiceRoutes) Delete(_ context.Context, serviceID uint64) error {
	f.routes = slices.DeleteFunc(f.routes, func(r domain.ServiceRoute) bool { return r.ServiceID == serviceID })
	return nil
}

type fakePreviewRoutes struct{ routes []domain.PreviewRoute }

func (f *fakePreviewRoutes) All(context.Context) ([]domain.PreviewRoute, error) {
	return slices.Clone(f.routes), nil
}
func (f *fakePreviewRoutes) Upsert(ctx context.Context, r domain.PreviewRoute) error {
	f.Delete(ctx, r.ApplicationID, r.Preview)
	f.routes = append(f.routes, r)
	return nil
}
func (f *fakePreviewRoutes) Delete(_ context.Context, applicationID uint64, preview int) error {
	f.routes = slices.DeleteFunc(f.routes, func(r domain.PreviewRoute) bool { return r.ApplicationID == applicationID && r.Preview == preview })
	return nil
}
func (f *fakePreviewRoutes) DeleteForApplication(_ context.Context, applicationID uint64) error {
	f.routes = slices.DeleteFunc(f.routes, func(r domain.PreviewRoute) bool { return r.ApplicationID == applicationID })
	return nil
}

type fakeSettings struct {
	all map[uint64]domain.RouteSettings
}

func (f *fakeSettings) All(context.Context) (map[uint64]domain.RouteSettings, error) {
	return f.all, nil
}
func (f *fakeSettings) Get(context.Context, uint64) (domain.RouteSettings, bool, error) {
	return domain.RouteSettings{}, false, nil
}
func (f *fakeSettings) Put(context.Context, domain.RouteSettings) error { return nil }
func (f *fakeSettings) Delete(context.Context, uint64) error            { return nil }

type fakeProxy struct{ applied [][]domain.Route }

func (f *fakeProxy) Ensure(context.Context) error { return nil }
func (f *fakeProxy) Apply(_ context.Context, routes []domain.Route) error {
	f.applied = append(f.applied, routes)
	return nil
}

// oneProxy serves every Server with the same Proxy.
type oneProxy struct{ p *fakeProxy }

func (o oneProxy) For(context.Context, uint64) (Proxy, error) { return o.p, nil }

// proxies has one fake Proxy per Server.
type proxies map[uint64]*fakeProxy

func (p proxies) For(_ context.Context, serverID uint64) (Proxy, error) {
	if _, ok := p[serverID]; !ok {
		p[serverID] = &fakeProxy{}
	}
	return p[serverID], nil
}

// upsertRoutes replaces a Route per Application, like the store.
type upsertRoutes struct{ fakeRoutes }

func (u *upsertRoutes) Upsert(_ context.Context, r domain.Route) error {
	u.routes = slices.DeleteFunc(u.routes, func(x domain.Route) bool { return x.ApplicationID == r.ApplicationID })
	u.routes = append(u.routes, r)
	return nil
}
func (u *upsertRoutes) Delete(_ context.Context, applicationID uint64) error {
	u.routes = slices.DeleteFunc(u.routes, func(x domain.Route) bool { return x.ApplicationID == applicationID })
	return nil
}

func TestRoutesPerServer(t *testing.T) {
	ctx := context.Background()
	routes := &upsertRoutes{fakeRoutes{routes: []domain.Route{{ApplicationID: 1, Domains: []string{"local.localhost"}, Container: "bakery-app-1-1", Port: 80}}}}
	svcRoutes := &fakeServiceRoutes{routes: []domain.ServiceRoute{{ServiceID: 3, Component: "web", Domains: []string{"shop.localhost"}, Container: "bakery-svc-3-web", Port: 80}}}
	ps := proxies{}
	s := NewService(routes, svcRoutes, &fakePreviewRoutes{}, &fakeSettings{}, ps)

	if err := s.SwitchRoute(ctx, domain.Route{ApplicationID: 2, ServerID: 7, Domains: []string{"remote.localhost"}, Container: "bakery-app-2-5", Port: 8080}); err != nil {
		t.Fatal(err)
	}
	if ps[0] != nil && len(ps[0].applied) != 0 {
		t.Fatalf("a Route on Server 7 applied the Local server: %+v", ps[0].applied)
	}
	remote := ps[7].applied
	if len(remote) != 1 || len(remote[0]) != 1 || remote[0][0].ApplicationID != 2 {
		t.Fatalf("Server 7 got %+v", remote)
	}

	// The Local server gets its own Routes and the Service routes, never
	// Server 7's.
	if err := s.EnsureProxy(ctx); err != nil {
		t.Fatal(err)
	}
	local := ps[0].applied[len(ps[0].applied)-1]
	if len(local) != 2 || local[0].ApplicationID != 1 || local[1].Container != "bakery-svc-3-web" {
		t.Fatalf("Local got %+v", local)
	}
	if ids, _ := s.RemoteServers(ctx); !slices.Equal(ids, []uint64{7}) {
		t.Fatalf("remote servers %v", ids)
	}

	// Dropping the remote Route applies Server 7 only, now empty.
	localApplies := len(ps[0].applied)
	if err := s.DropRoute(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if len(ps[0].applied) != localApplies || len(ps[7].applied[len(ps[7].applied)-1]) != 0 {
		t.Fatalf("drop: local %d applies, server 7 last %+v", len(ps[0].applied), ps[7].applied[len(ps[7].applied)-1])
	}
}

func TestServiceRoutes(t *testing.T) {
	ctx := context.Background()
	apps := &fakeRoutes{routes: []domain.Route{{ApplicationID: 1, Domains: []string{"app.localhost"}, Container: "bakery-app-1-1", Port: 80}}}
	svcRoutes := &fakeServiceRoutes{}
	// Settings for application id 0 must never reach a Service route.
	settings := &fakeSettings{all: map[uint64]domain.RouteSettings{0: {Redirect: domain.NonWww}}}
	proxy := &fakeProxy{}
	s := NewService(apps, svcRoutes, &fakePreviewRoutes{}, settings, oneProxy{proxy})

	web := domain.ServiceRoute{ServiceID: 3, Component: "web", Domains: []string{"shop.localhost"}, Container: "bakery-svc-3-web", Port: 80}
	admin := domain.ServiceRoute{ServiceID: 3, Component: "admin", Domains: []string{"shop-admin.localhost"}, Container: "bakery-svc-3-admin", Port: 8080}
	if err := s.SetServiceRoutes(ctx, 3, []domain.ServiceRoute{web, admin}); err != nil {
		t.Fatal(err)
	}
	last := proxy.applied[len(proxy.applied)-1]
	if len(last) != 3 || last[1].Container != "bakery-svc-3-web" || last[2].Settings.Redirect != domain.Both {
		t.Fatalf("applied %+v", last)
	}
	if err := s.SetServiceRoutes(ctx, 3, []domain.ServiceRoute{web}); err != nil {
		t.Fatal(err)
	}
	if last = proxy.applied[len(proxy.applied)-1]; len(last) != 2 {
		t.Fatalf("replace kept the old set: %+v", last)
	}
	if err := s.DropServiceRoutes(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if last = proxy.applied[len(proxy.applied)-1]; len(last) != 1 || last[0].ApplicationID != 1 {
		t.Fatalf("after drop %+v", last)
	}
}

func TestPreviewRoutes(t *testing.T) {
	ctx := context.Background()
	routes := &upsertRoutes{}
	previews := &fakePreviewRoutes{}
	settings := &fakeSettings{all: map[uint64]domain.RouteSettings{1: {
		ApplicationID: 1, Redirect: domain.Www,
		BasicAuth: domain.BasicAuth{Enabled: true, Username: "u", PasswordHash: "h"},
	}}}
	ps := proxies{}
	s := NewService(routes, &fakeServiceRoutes{}, previews, settings, ps)
	if err := s.SwitchRoute(ctx, domain.Route{ApplicationID: 1, Domains: []string{"app.localhost"}, Container: "c1", Port: 80}); err != nil {
		t.Fatal(err)
	}
	if err := s.SwitchPreviewRoute(ctx, domain.PreviewRoute{ApplicationID: 1, Preview: 7, Domains: []string{"pr-7.app.localhost"}, Container: "c7", Port: 80}); err != nil {
		t.Fatal(err)
	}
	last := ps[0].applied[len(ps[0].applied)-1]
	if len(last) != 2 {
		t.Fatalf("applied %+v", last)
	}
	p := last[1]
	if !p.Derived || p.Container != "c7" || p.Settings.Redirect != domain.Both || !p.Settings.BasicAuth.Enabled {
		t.Fatalf("preview route %+v", p)
	}
	if err := s.DropPreviewRoute(ctx, 1, 7); err != nil {
		t.Fatal(err)
	}
	if last := ps[0].applied[len(ps[0].applied)-1]; len(last) != 1 {
		t.Fatalf("after drop %+v", last)
	}
	// Dropping again does nothing.
	n := len(ps[0].applied)
	if err := s.DropPreviewRoute(ctx, 1, 7); err != nil || len(ps[0].applied) != n {
		t.Fatalf("second drop: %v, %d applies", err, len(ps[0].applied)-n)
	}
	// Deleting the Application drops its Preview routes too.
	_ = s.SwitchPreviewRoute(ctx, domain.PreviewRoute{ApplicationID: 1, Preview: 8, Domains: []string{"pr-8.app.localhost"}, Container: "c8", Port: 80})
	if err := s.DropRoute(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if len(previews.routes) != 0 || len(ps[0].applied[len(ps[0].applied)-1]) != 0 {
		t.Fatalf("after DropRoute: %+v", previews.routes)
	}
}
