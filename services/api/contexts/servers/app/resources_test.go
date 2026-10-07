package app

import (
	"context"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

func TestResourcesNamesTheirPlaceAndSorts(t *testing.T) {
	store := newMemStore()
	s := NewService(store, fakeKey, nil)
	local, _ := store.Create(context.Background(), domain.Server{Name: "localhost", Kind: domain.Local})
	var asked uint64 = 99
	s.OnResources("application", func(_ context.Context, serverID uint64, _ []ResourceProject) ([]Resource, error) {
		asked = serverID
		return []Resource{{ID: 1, Name: "web", EnvironmentID: 10, Status: "running"}, {ID: 2, Name: "hidden", EnvironmentID: 30}}, nil
	})
	s.OnResources("database", func(context.Context, uint64, []ResourceProject) ([]Resource, error) {
		return []Resource{{ID: 3, Name: "db", EnvironmentID: 20, Status: "exited"}}, nil
	})
	projects := []ResourceProject{{ID: 5, Name: "shop", Environments: map[uint64]string{10: "production", 20: "staging"}}}

	got, err := s.Resources(context.Background(), local.ID, projects)
	if err != nil {
		t.Fatal(err)
	}
	if asked != 0 {
		t.Fatalf("the Local server is asked for as %d, want 0", asked)
	}
	want := []Resource{
		{Type: "database", ID: 3, Name: "db", ProjectID: 5, Project: "shop", EnvironmentID: 20, Environment: "staging", Status: "exited"},
		{Type: "application", ID: 1, Name: "web", ProjectID: 5, Project: "shop", EnvironmentID: 10, Environment: "production", Status: "running"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestResourcesOfAMissingServer(t *testing.T) {
	s := NewService(newMemStore(), fakeKey, nil)
	if _, err := s.Resources(context.Background(), 7, nil); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
