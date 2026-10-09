package bakery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestRunRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/desktop/runs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"runs":[{"id":4,"status":"queued","guild":{"id":1,"name":"Bakers"},"agent":{"id":2,"name":"Ada","icon":""},"issue":{"id":9,"identifier":"BAK-9","title":"Fix it"},"prompt":"Fix it","next_seq":1}]}`))
	})
	mux.HandleFunc("POST /api/runs/4/claim", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"a running run cannot be claimed"}`))
	})
	mux.HandleFunc("POST /api/runs/4/events", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Events []RunEvent `json:"events"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if len(in.Events) != 2 || in.Events[1].Seq != 2 || in.Events[0].Kind != "init" || string(in.Events[0].Payload) != `{"session_id":"s"}` {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		_, _ = w.Write([]byte(`{"run":{"id":4,"status":"running","next_seq":3,"session_id":"s"}}`))
	})
	mux.HandleFunc("POST /api/runs/4/finish", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		usage, _ := in["usage"].(map[string]any)
		if in["status"] != "succeeded" || in["exit_code"] != float64(0) || usage["turns"] != float64(3) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		_, _ = w.Write([]byte(`{"run":{"id":4,"status":"succeeded","next_seq":3}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "bky_desk_x")
	ctx := context.Background()
	runs, err := c.DesktopRuns(ctx)
	if err != nil || len(runs) != 1 || runs[0].Issue.Identifier != "BAK-9" || runs[0].Agent.Name != "Ada" {
		t.Fatalf("runs %+v, %v", runs, err)
	}
	if _, err := c.ClaimRun(ctx, 4); !IsStatus(err, http.StatusConflict) || err.Error() != "a running run cannot be claimed" {
		t.Fatalf("claim: %v", err)
	}
	st, err := c.AppendRunEvents(ctx, 4, []RunEvent{{Seq: 1, Kind: "init", Payload: json.RawMessage(`{"session_id":"s"}`)}, {Seq: 2, Kind: "assistant", Payload: json.RawMessage(`{"text":"hi"}`)}})
	if err != nil || st.NextSeq != 3 || st.SessionID != "s" {
		t.Fatalf("events %+v, %v", st, err)
	}
	code := 0
	if st, err := c.FinishRun(ctx, 4, Finish{Status: "succeeded", ExitCode: &code, Usage: Usage{Turns: 3}}); err != nil || st.Status != "succeeded" {
		t.Fatalf("finish %+v, %v", st, err)
	}
}

func TestWatchRuns(t *testing.T) {
	streamBackoffMin = 10 * time.Millisecond
	var connects int
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/desktop/runs/stream", func(w http.ResponseWriter, r *http.Request) {
		connects++
		if r.Header.Get("Authorization") != "Bearer bky_desk_x" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if connects == 1 {
			// The first connection drops at once; the watcher comes back.
			_, _ = w.Write([]byte(": ping\n\n"))
			return
		}
		_, _ = w.Write([]byte("event: runs\ndata: {\"runs\":[{\"id\":4,\"status\":\"queued\"}]}\n\n: ping\n\nevent: cancel\ndata: {\"run_id\":7}\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan string, 4)
	done := make(chan error, 1)
	go func() {
		done <- New(srv.URL, "bky_desk_x").WatchRuns(ctx, RunsWatcher{
			Runs:   func(rs []DesktopRun) { got <- fmt.Sprintf("runs %d %s", rs[0].ID, rs[0].Status) },
			Cancel: func(id uint64) { got <- fmt.Sprintf("cancel %d", id) },
		})
	}()
	for _, want := range []string{"runs 4 queued", "cancel 7"} {
		select {
		case g := <-got:
			if g != want {
				t.Fatalf("got %q, want %q", g, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no %q", want)
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("watch ended with %v", err)
	}
	if err := New(srv.URL, "bky_desk_wrong").WatchRuns(context.Background(), RunsWatcher{}); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("a refused key: %v", err)
	}
}

func TestRunsAndRunEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Bakery-Guild") != "1" || r.URL.Query().Get("agent") != "2" || r.URL.Query().Get("limit") != "20" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"runs":[{"id":4,"agent":{"id":2,"name":"Ada"},"status":"succeeded","usage":{"turns":3},"can_cancel":false}]}`))
	})
	mux.HandleFunc("GET /api/runs/4/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Bakery-Guild") != "1" || r.URL.Query().Get("after") != "2" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"events":[{"seq":3,"kind":"result","payload":{"subtype":"success"}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "bky_desk_x")
	ctx := context.Background()
	runs, err := c.Runs(ctx, 1, 2, 20)
	if err != nil || len(runs) != 1 || runs[0].ID != 4 || runs[0].Status != "succeeded" || runs[0].Usage.Turns != 3 {
		t.Fatalf("runs %+v, %v", runs, err)
	}
	events, err := c.RunEvents(ctx, 1, 4, 2)
	if err != nil || len(events) != 1 || events[0].Seq != 3 || events[0].Kind != "result" {
		t.Fatalf("events %+v, %v", events, err)
	}
}

func TestFollowRunStream(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runs/4/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bky_desk_x" || r.Header.Get("Bakery-Guild") != "1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: event\ndata: {\"seq\":1,\"kind\":\"assistant\",\"payload\":{\"text\":\"hi\"}}\n\n: ping\n\nevent: end\ndata: {\"status\":\"succeeded\"}\n\n"))
		w.(http.Flusher).Flush()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	var got []RunStreamUpdate
	err := New(srv.URL, "bky_desk_x").FollowRunStream(context.Background(), 1, 4, func(u RunStreamUpdate) { got = append(got, u) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != "event" || got[0].Event.Seq != 1 || got[0].Event.Kind != "assistant" || got[1].Kind != "end" || got[1].Status != "succeeded" {
		t.Fatalf("got %+v", got)
	}
	if err := New(srv.URL, "bky_desk_wrong").FollowRunStream(context.Background(), 1, 4, nil); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("a refused key: %v", err)
	}
}

func TestDesktopRunSkills(t *testing.T) {
	var r DesktopRun
	if err := json.Unmarshal([]byte(`{"id":4,"run_key":"bky_run_x","workspace":null,"skills":[{"slug":"release-notes","files":[
		{"path":"SKILL.md","content":"---\nname: Release notes\n---\n","encoding":"utf8","executable":false},
		{"path":"bin/run","content":"AAE=","encoding":"base64","executable":true}]}]}`), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Skills) != 1 || r.Skills[0].Slug != "release-notes" || len(r.Skills[0].Files) != 2 {
		t.Fatalf("skills: %+v", r.Skills)
	}
	if f := r.Skills[0].Files[1]; f.Path != "bin/run" || f.Encoding != "base64" || f.Content != "AAE=" || !f.Executable {
		t.Fatalf("file: %+v", f)
	}
}
