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

func TestServiceRoutes(t *testing.T) {
	ctx := context.Background()
	apps := &fakeRoutes{routes: []domain.Route{{ApplicationID: 1, Domains: []string{"app.localhost"}, Container: "bakery-app-1-1", Port: 80}}}
	svcRoutes := &fakeServiceRoutes{}
	// Settings for application id 0 must never reach a Service route.
	settings := &fakeSettings{all: map[uint64]domain.RouteSettings{0: {WwwRedirect: domain.ToApex}}}
	proxy := &fakeProxy{}
	s := NewService(apps, svcRoutes, settings, proxy)

	web := domain.ServiceRoute{ServiceID: 3, Component: "web", Domains: []string{"shop.localhost"}, Container: "bakery-svc-3-web", Port: 80}
	admin := domain.ServiceRoute{ServiceID: 3, Component: "admin", Domains: []string{"shop-admin.localhost"}, Container: "bakery-svc-3-admin", Port: 8080}
	if err := s.SetServiceRoutes(ctx, 3, []domain.ServiceRoute{web, admin}); err != nil {
		t.Fatal(err)
	}
	last := proxy.applied[len(proxy.applied)-1]
	if len(last) != 3 || last[1].Container != "bakery-svc-3-web" || last[2].Settings.WwwRedirect != domain.WwwOff {
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
