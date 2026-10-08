package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jevido/bakery/apps/desktop/store"
)

// fakeSignIns is a Bakery whose one sign-in is approved after approveAfter
// polls, and which counts sign-outs.
func fakeSignIns(t *testing.T, approveAfter int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var polls, signOuts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/desktop-sign-ins", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		exp := time.Now().Add(time.Minute).UTC().Format(time.RFC3339)
		_, _ = w.Write([]byte(`{"id":5,"approval_url":"http://x/#/desktop-sign-in/5","expires_at":"` + exp + `","poll_interval_ms":500}`))
	})
	mux.HandleFunc("GET /api/desktop-sign-ins/5", func(w http.ResponseWriter, r *http.Request) {
		status := "pending"
		if polls.Add(1) >= approveAfter {
			status = "approved"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer bky_desk_") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"member":{"id":1,"name":"Owner","email":"owner@example.com"}}`))
	})
	mux.HandleFunc("GET /api/desktops", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":9,"current":true}]`))
	})
	mux.HandleFunc("POST /api/desktops/current/sign-out", func(w http.ResponseWriter, r *http.Request) {
		signOuts.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &signOuts
}

func TestConnectAndDisconnect(t *testing.T) {
	srv, signOuts := fakeSignIns(t, 2)
	path := filepath.Join(t.TempDir(), "bakeries.json")
	events := NewEvents()
	got, stop := events.Subscribe()
	defer stop()
	var opened string
	d := NewDesktop(events, store.New(path), func(u string) error { opened = u; return nil }, nil)

	start, err := d.Connect(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if start.Address != srv.URL || opened != start.ApprovalURL {
		t.Fatalf("start %+v, opened %q", start, opened)
	}
	deadline := time.After(10 * time.Second)
	for done := false; !done; {
		select {
		case e := <-got:
			if s, ok := e.Data.(ConnectState); ok && e.Name == "connect" {
				if s.Status != connectApproved {
					t.Fatalf("connect ended %+v", s)
				}
				done = true
			}
		case <-deadline:
			t.Fatal("no approved event")
		}
	}
	if s, _ := d.ConnectStatus(start.ID); s.Status != connectApproved {
		t.Fatalf("status %+v", s)
	}
	bs, err := d.Bakeries()
	if err != nil || len(bs) != 1 || !bs[0].Active || bs[0].Member.Name != "Owner" {
		t.Fatalf("bakeries %+v, %v", bs, err)
	}
	f, _ := store.New(path).Load()
	if b, _ := f.Find(srv.URL); b.DesktopID != 9 || !strings.HasPrefix(b.Key, "bky_desk_") {
		t.Fatalf("stored %+v", b)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}

	srv.Close() // Disconnect forgets the key even when the Bakery is gone.
	if err := d.Disconnect(srv.URL); err != nil {
		t.Fatal(err)
	}
	if bs, _ := d.Bakeries(); len(bs) != 0 {
		t.Fatalf("after disconnect %+v", bs)
	}
	if signOuts.Load() != 0 {
		t.Fatalf("a closed Bakery counted a sign-out")
	}
}

func TestDisconnectSignsOut(t *testing.T) {
	srv, signOuts := fakeSignIns(t, 1)
	path := filepath.Join(t.TempDir(), "bakeries.json")
	_ = store.New(path).Put(store.Bakery{Address: srv.URL, Key: "bky_desk_x"})
	d := NewDesktop(NewEvents(), store.New(path), nil, nil)
	if err := d.Disconnect(srv.URL); err != nil {
		t.Fatal(err)
	}
	if signOuts.Load() != 1 {
		t.Fatalf("sign-outs %d", signOuts.Load())
	}
	if err := d.Disconnect(srv.URL); err != store.ErrUnknown {
		t.Fatalf("again: %v", err)
	}
}

func TestConnectRefusesBadAddress(t *testing.T) {
	d := NewDesktop(NewEvents(), store.New(filepath.Join(t.TempDir(), "b.json")), nil, nil)
	if _, err := d.Connect("ftp://x"); err == nil {
		t.Fatal("want an error")
	}
}
