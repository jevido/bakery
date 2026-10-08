package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testServer(t *testing.T) (*httptest.Server, *Events) {
	t.Helper()
	events := NewEvents()
	assets := fstest.MapFS{"index.html": {Data: []byte("<title>The Bakery</title>")}}
	srv := httptest.NewServer(newServer(NewDesktop(events), assets))
	t.Cleanup(srv.Close)
	return srv, events
}

func TestRPC(t *testing.T) {
	srv, _ := testServer(t)
	tests := []struct {
		name, method, body string
		status             int
		want               string
	}{
		{"version", "Version", "[]", http.StatusOK, `{"result":"dev"}`},
		{"unknown method", "Nope", "[]", http.StatusNotFound, `{"error":"unknown method"}`},
		{"unexported method", "methods", "[]", http.StatusNotFound, `{"error":"unknown method"}`},
		{"not an array", "Version", "{}", http.StatusBadRequest, `{"error":"the body must be a JSON array of arguments"}`},
		{"too many arguments", "Version", `["x"]`, http.StatusBadRequest, `{"error":"bad arguments: want 0, got 1"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Post(srv.URL+"/rpc/"+tt.method, "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			var got json.RawMessage
			if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tt.status || string(got) != tt.want {
				t.Errorf("got %d %s, want %d %s", res.StatusCode, got, tt.status, tt.want)
			}
		})
	}
}

func TestRPCGetIsNotACall(t *testing.T) {
	srv, _ := testServer(t)
	res, err := http.Get(srv.URL + "/rpc/Version")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Errorf("GET /rpc/Version answered 200")
	}
}

func TestFrontend(t *testing.T) {
	srv, _ := testServer(t)
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var b strings.Builder
	_, _ = bufio.NewReader(res.Body).WriteTo(&b)
	if res.StatusCode != http.StatusOK || !strings.Contains(b.String(), "The Bakery") {
		t.Errorf("GET / = %d %q", res.StatusCode, b.String())
	}
}

func TestEventsStream(t *testing.T) {
	srv, events := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/rpc/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type %q", ct)
	}
	lines := bufio.NewScanner(res.Body)
	next := func() string {
		if !lines.Scan() {
			t.Fatal("stream ended")
		}
		return lines.Text()
	}
	if got := next() + "|" + next(); got != "event: ping|data: null" {
		t.Fatalf("first message %q", got)
	}
	next()
	events.Emit("hello", map[string]int{"n": 1})
	if got := next() + "|" + next(); got != `event: hello|data: {"n":1}` {
		t.Fatalf("emitted message %q", got)
	}
}
