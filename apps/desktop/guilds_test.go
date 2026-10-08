package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
	"github.com/jevido/bakery/apps/desktop/store"
)

// fakeGuilds is a Bakery with two Guilds whose Agents answer only with the
// right key and Bakery-Guild; set changes guild 7's Agents, revoke signs
// the key out.
type fakeGuilds struct {
	mu      sync.Mutex
	agents  []bakery.Agent
	revoked bool
}

func (f *fakeGuilds) set(agents ...bakery.Agent) {
	f.mu.Lock()
	f.agents = agents
	f.mu.Unlock()
}

func (f *fakeGuilds) revoke() {
	f.mu.Lock()
	f.revoked = true
	f.mu.Unlock()
}

func (f *fakeGuilds) serve(t *testing.T) *httptest.Server {
	t.Helper()
	keyed := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			revoked := f.revoked
			f.mu.Unlock()
			if revoked || r.Header.Get("Authorization") != "Bearer bky_desk_k" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/guilds", keyed(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"guilds":[{"id":7,"name":"Default","issue_prefix":"DEF"},{"id":8,"name":"Second","issue_prefix":"SEC"}]}`))
	}))
	mux.HandleFunc("GET /api/agents", keyed(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Bakery-Guild") != "7" || r.URL.Query().Get("status") != "all" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"agents": f.agents})
	}))
	mux.HandleFunc("GET /api/agents/{id}", keyed(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Bakery-Guild") != "7" || r.PathValue("id") != "3" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"agent":{"id":3,"name":"Ada","status":"idle","roles":[{"id":2,"name":"Member"}]}}`))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func connected(t *testing.T, address string) (*Desktop, <-chan Event) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bakeries.json")
	if err := store.New(path).Put(store.Bakery{Address: address, Key: "bky_desk_k"}); err != nil {
		t.Fatal(err)
	}
	events := NewEvents()
	got, stop := events.Subscribe()
	t.Cleanup(stop)
	return NewDesktop(events, store.New(path), nil), got
}

// next is the next event named name, skipping others.
func next(t *testing.T, got <-chan Event, name string) Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-got:
			if e.Name == name {
				return e
			}
		case <-deadline:
			t.Fatalf("no %s event", name)
		}
	}
}

func TestGuildsAndAgentsWithBakeryGuild(t *testing.T) {
	fake := &fakeGuilds{}
	fake.set(bakery.Agent{ID: 3, Name: "Ada", Status: "idle"})
	srv := fake.serve(t)
	d, _ := connected(t, srv.URL)

	guilds, err := d.Guilds(srv.URL)
	if err != nil || len(guilds) != 2 || guilds[0].Name != "Default" {
		t.Fatalf("guilds %+v, %v", guilds, err)
	}
	agents, err := d.Agents(srv.URL, 7, "all")
	if err != nil || len(agents) != 1 || agents[0].Name != "Ada" {
		t.Fatalf("agents %+v, %v", agents, err)
	}
	// Guild 8 is refused by the fake: the header carries the Guild asked for.
	if _, err := d.Agents(srv.URL, 8, "all"); err == nil {
		t.Fatal("guild 8's Agents answered")
	}
	a, err := d.Agent(srv.URL, 7, 3)
	if err != nil || len(a.Roles) != 1 || a.Roles[0].Name != "Member" {
		t.Fatalf("agent %+v, %v", a, err)
	}
	if _, err := d.Agent(srv.URL, 8, 3); err != bakery.ErrNotFound {
		t.Fatalf("agent in guild 8: %v", err)
	}
}

func TestRefreshSendsWhatChanged(t *testing.T) {
	fake := &fakeGuilds{}
	fake.set(bakery.Agent{ID: 3, Name: "Ada", Status: "idle"})
	srv := fake.serve(t)
	d, got := connected(t, srv.URL)
	if _, err := d.Guilds(srv.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Agents(srv.URL, 7, "all"); err != nil {
		t.Fatal(err)
	}

	// Nothing changed: no event. Then a hire: one agents event with it.
	d.refresh()
	fake.set(bakery.Agent{ID: 3, Name: "Ada", Status: "idle"}, bakery.Agent{ID: 4, Name: "Grace", Status: "pending_approval"})
	d.refresh()
	e := next(t, got, "agents")
	ae, ok := e.Data.(AgentsEvent)
	if !ok || ae.GuildID != 7 || ae.Status != "all" || len(ae.Agents) != 2 || ae.Agents[1].Name != "Grace" {
		t.Fatalf("event %+v", e.Data)
	}
	select {
	case e := <-got:
		t.Fatalf("unexpected %s event", e.Name)
	default:
	}
	// Asked again, the list has the hire.
	if agents, _ := d.Agents(srv.URL, 7, "all"); len(agents) != 2 {
		t.Fatalf("agents %+v", agents)
	}

	// A 401 marks the Bakery signed out and stops the refresh.
	fake.revoke()
	d.refresh()
	next(t, got, "bakeries")
	if bs, _ := d.Bakeries(); !bs[0].SignedOut {
		t.Fatalf("not signed out: %+v", bs)
	}
	if d.shown != (shown{}) {
		t.Fatalf("still refreshing %+v", d.shown)
	}
	// The cached Guilds are fresh, but a signed-out Bakery answers nothing.
	if _, err := d.Guilds(srv.URL); err != bakery.ErrSignedOut {
		t.Fatalf("guilds after sign-out: %v", err)
	}
}
