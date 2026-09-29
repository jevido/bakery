package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/projects/domain"
)

// fakeStore implements only what CreateApplication and UpdateApplication
// touch; anything else panics through the nil embedded Store.
type fakeStore struct {
	Store
	app domain.Application
}

func (f *fakeStore) Environment(context.Context, uint64) (domain.Environment, bool, error) {
	return domain.Environment{ID: 1, ProjectID: 1, Name: domain.DefaultEnvironment}, true, nil
}
func (f *fakeStore) SlugTaken(context.Context, string) (bool, error) { return false, nil }
func (f *fakeStore) DomainTaken(context.Context, string, uint64) (bool, error) {
	return false, nil
}
func (f *fakeStore) CreateApplication(_ context.Context, a domain.Application) (domain.Application, error) {
	a.ID = 1
	f.app = a
	return a, nil
}
func (f *fakeStore) Application(context.Context, uint64) (domain.Application, bool, error) {
	return f.app, f.app.ID != 0, nil
}
func (f *fakeStore) UpdateApplication(_ context.Context, a domain.Application) error {
	f.app = a
	return nil
}

func TestDashboardDomainIsReserved(t *testing.T) {
	ctx := context.Background()
	s := NewService(&fakeStore{}, "example.com", "bakery.example.com")
	in := domain.ApplicationInput{Name: "web", GitURL: "https://example.com/r.git", Port: 80}

	in.Domain = "Bakery.Example.com"
	_, err := s.CreateApplication(ctx, 1, in)
	var fe *domain.FieldError
	if !errors.As(err, &fe) || fe.Field != "domain" {
		t.Fatalf("create with the dashboard domain: want a domain FieldError, got %v", err)
	}

	in.Domain = ""
	a, err := s.CreateApplication(ctx, 1, in)
	if err != nil || a.Domain != "web.example.com" {
		t.Fatalf("create with the default domain: %v, %q", err, a.Domain)
	}

	in.Domain = "bakery.example.com"
	if _, err := s.UpdateApplication(ctx, a.ID, in); !errors.As(err, &fe) || fe.Field != "domain" {
		t.Fatalf("update to the dashboard domain: want a domain FieldError, got %v", err)
	}
}
