package bakery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAddress(t *testing.T) {
	for in, want := range map[string]string{
		"bakery.example.com":              "https://bakery.example.com",
		" https://Bakery.Example.com/ ":   "https://bakery.example.com",
		"http://127.0.0.1:4930":           "http://127.0.0.1:4930",
		"https://example.com/bakery/?x#y": "https://example.com/bakery",
	} {
		got, err := NormalizeAddress(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "ftp://example.com", "https://"} {
		if _, err := NormalizeAddress(in); err == nil {
			t.Errorf("%q: want an error", in)
		}
	}
}

// fakeBakery answers the sign-in routes and checks what the client sends.
func fakeBakery(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var keyHash string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/desktop-sign-ins", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		if !strings.HasPrefix(in["token"], signInSecretPrefix) || len(in["token"]) != len(signInSecretPrefix)+48 || in["client_name"] != "laptop" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		keyHash = in["desktop_key_hash"]
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":5,"approval_url":"http://x/#/desktop-sign-in/5?token=t","expires_at":"2026-10-08T12:00:00Z","poll_interval_ms":1000}`))
	})
	mux.HandleFunc("GET /api/desktop-sign-ins/5", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Query().Get("token"), signInSecretPrefix) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"approved"}`))
	})
	authed := func(r *http.Request) bool {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		sum := sha256.Sum256([]byte(key))
		return keyHash != "" && hex.EncodeToString(sum[:]) == keyHash
	}
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"member":{"id":1,"name":"Owner","email":"owner@example.com"}}`))
	})
	mux.HandleFunc("GET /api/desktops", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":3,"current":false},{"id":9,"current":true}]`))
	})
	mux.HandleFunc("POST /api/desktops/current/sign-out", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		keyHash = ""
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &keyHash
}

func TestSignInFlow(t *testing.T) {
	srv, _ := fakeBakery(t)
	ctx := context.Background()
	c := New(srv.URL, "")
	s, err := c.StartSignIn(ctx, "laptop")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != 5 || s.PollInterval.Seconds() != 1 || !strings.HasPrefix(s.Key, desktopKeyPrefix) {
		t.Fatalf("%+v", s)
	}
	if status, err := c.SignInStatus(ctx, s); err != nil || status != "approved" {
		t.Fatalf("status %q, %v", status, err)
	}
	if _, err := c.SignInStatus(ctx, SignIn{ID: 6, Token: s.Token}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown sign-in: %v", err)
	}
	keyed := New(srv.URL, s.Key)
	me, err := keyed.Me(ctx)
	if err != nil || me.Name != "Owner" {
		t.Fatalf("me %+v, %v", me, err)
	}
	if id, err := keyed.DesktopID(ctx); err != nil || id != 9 {
		t.Fatalf("desktop id %d, %v", id, err)
	}
	if err := keyed.SignOut(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := keyed.Me(ctx); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("after sign-out: %v", err)
	}
}

func TestErrorMessageAndGuildHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Bakery-Guild") != "4" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"not a member of this guild"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "bky_desk_x")
	err := c.do(context.Background(), http.MethodGet, "/api/agents", 2, nil, nil)
	var e *Error
	if !errors.As(err, &e) || e.Status != 403 || e.Message != "not a member of this guild" {
		t.Fatalf("%v", err)
	}
	if err := c.do(context.Background(), http.MethodGet, "/api/agents", 4, nil, nil); err != nil {
		t.Fatal(err)
	}
}
