package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
	"github.com/jevido/bakery/apps/desktop/store"
)

// standin is the claude stand-in, built once for every test.
var standin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "runner-standin")
	if err != nil {
		panic(err)
	}
	standin = filepath.Join(dir, "claude-standin")
	if out, err := exec.Command("go", "build", "-o", standin, "../standin/claude").CombinedOutput(); err != nil {
		panic(fmt.Sprintf("building the stand-in: %v\n%s", err, out))
	}
	flushEvery, retryEvery, killAfter = 20*time.Millisecond, 50*time.Millisecond, time.Second
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestClaudeEnvRemovesAPIKeys(t *testing.T) {
	got := claudeEnv([]string{"PATH=/bin", "ANTHROPIC_API_KEY=sk-x", "HOME=/h", "ANTHROPIC_AUTH_TOKEN=t", "anthropic_api_key=y", "ANTHROPIC_MODEL=m"})
	want := []string{"PATH=/bin", "HOME=/h", "ANTHROPIC_MODEL=m"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// standinLines runs the stand-in with prompt as the Runner would and
// answers its stdout and stderr lines and its exit code.
func standinLines(t *testing.T, prompt string) (stdout, stderr []string, code int) {
	t.Helper()
	cmd := exec.Command(standin, claudeArgs...)
	cmd.Env = append(claudeEnv(os.Environ()), "BAKERY_STANDIN_DELAY=0")
	cmd.Stdin = strings.NewReader(prompt)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	split := func(s string) []string {
		var lines []string
		sc := bufio.NewScanner(strings.NewReader(s))
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		return lines
	}
	return split(out.String()), split(errOut.String()), code
}

func kinds(events []Event) string {
	ks := make([]string, len(events))
	for i, e := range events {
		ks[i] = e.Kind
	}
	return strings.Join(ks, " ")
}

func TestTranscriptOfASuccessfulRun(t *testing.T) {
	stdout, _, code := standinLines(t, "Fix the login page\n\nIt is broken.")
	tr := &Transcript{}
	var events []Event
	for _, l := range stdout {
		events = append(events, tr.Stdout(l)...)
	}
	if got, want := kinds(events), "init assistant tool_call tool_result thinking assistant result"; got != want {
		t.Fatalf("kinds %q, want %q", got, want)
	}
	if tr.SessionID == "" || tr.Model != "claude-standin" {
		t.Fatalf("session %q model %q", tr.SessionID, tr.Model)
	}
	var call toolCallPayload
	_ = json.Unmarshal(events[2].Payload, &call)
	if call.Name != "Read" || call.ID != "toolu_standin_1" || !strings.Contains(string(call.Input), "README.md") {
		t.Fatalf("tool_call %s", events[2].Payload)
	}
	var res toolResultPayload
	_ = json.Unmarshal(events[3].Payload, &res)
	if res.ToolUseID != "toolu_standin_1" || !strings.Contains(res.Content, "claude stand-in") || res.IsError {
		t.Fatalf("tool_result %s", events[3].Payload)
	}
	var text textPayload
	_ = json.Unmarshal(events[5].Payload, &text)
	if text.Text != "Done with Fix the login page." {
		t.Fatalf("assistant %s", events[5].Payload)
	}
	f := tr.Finish(code)
	if f.Status != "succeeded" || f.Error != "" || *f.ExitCode != 0 {
		t.Fatalf("finish %+v", f)
	}
	if f.Usage.Turns != 4 || f.Usage.InputTokens != 4800 || f.Usage.CachedInputTokens != 38400 || f.Usage.OutputTokens != 720 || f.Usage.CostEquivalentUSD <= 0 {
		t.Fatalf("usage %+v", f.Usage)
	}
}

func TestTranscriptOfFailedRuns(t *testing.T) {
	stdout, _, code := standinLines(t, "Do it [fail]")
	tr := &Transcript{}
	for _, l := range stdout {
		tr.Stdout(l)
	}
	if f := tr.Finish(code); f.Status != "failed" || !strings.Contains(f.Error, "failed on purpose") || *f.ExitCode != 1 {
		t.Fatalf("[fail] finish %+v", f)
	}

	stdout, stderr, code := standinLines(t, "Do it [crash]")
	tr = &Transcript{}
	var events []Event
	for _, l := range stdout {
		events = append(events, tr.Stdout(l)...)
	}
	for _, l := range stderr {
		events = append(events, tr.Stderr(l)...)
	}
	if got := kinds(events); got != "init assistant stderr" {
		t.Fatalf("[crash] kinds %q", got)
	}
	f := tr.Finish(code)
	if f.Status != "failed" || !strings.Contains(f.Error, "code 1 and no result") || !strings.Contains(f.Error, "crashed on purpose") {
		t.Fatalf("[crash] finish %+v", f)
	}
}

func TestTranscriptOfOddLines(t *testing.T) {
	tr := &Transcript{}
	if got := kinds(tr.Stdout("not json at all")); got != "system" {
		t.Fatalf("not JSON: %q", got)
	}
	// Lines the real CLI prints that are no Run event (seen in the run by hand).
	for _, l := range []string{
		`{"type":"system","subtype":"compact_boundary"}`,
		`{"type":"system","subtype":"hook_started","hook_name":"SessionStart:startup","hook_event":"SessionStart"}`,
		`{"type":"system","subtype":"hook_response","hook_name":"SessionStart:startup","exit_code":0,"outcome":"success"}`,
		`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed_warning","rateLimitType":"seven_day","utilization":0.55}}`,
	} {
		if got := tr.Stdout(l); len(got) != 0 {
			t.Fatalf("%s: %v", l, got)
		}
	}
	big := strings.Repeat("x", maxToolResult+10)
	line, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": []any{map[string]any{"type": "text", "text": big}}},
	}}})
	events := tr.Stdout(string(line))
	var p toolResultPayload
	_ = json.Unmarshal(events[0].Payload, &p)
	if len(p.Content) != maxToolResult || !p.Truncated {
		t.Fatalf("tool result cut to %d, truncated %v", len(p.Content), p.Truncated)
	}
}

// fakeBakery is a Bakery with one Run for this Desktop: it hands it out on
// the list and the stream, takes one claim, checks the seqs, and records
// what the Runner reports.
type fakeBakery struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	run      bakery.DesktopRun
	claims   int
	events   []bakery.RunEvent
	leases   int
	finished *bakery.Finish
	cancel   chan struct{}
}

func newFakeBakery(t *testing.T, prompt string) *fakeBakery {
	f := &fakeBakery{t: t, cancel: make(chan struct{}),
		run: bakery.DesktopRun{ID: 41, Status: "queued", Guild: bakery.Named{ID: 1, Name: "Bakers"}, Agent: bakery.RunAgent{ID: 3, Name: "Ada"}, Prompt: prompt, NextSeq: 1}}
	mux := http.NewServeMux()
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer bky_desk_test" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	list := func() []bakery.DesktopRun {
		if f.run.Status == "queued" {
			return []bakery.DesktopRun{f.run}
		}
		return nil
	}
	mux.HandleFunc("GET /api/desktop/runs", authed(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, 200, map[string]any{"runs": list()})
	}))
	mux.HandleFunc("GET /api/desktop/runs/stream", authed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f.mu.Lock()
		b, _ := json.Marshal(map[string]any{"runs": list()})
		f.mu.Unlock()
		fmt.Fprintf(w, "event: runs\ndata: %s\n\n", b)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-f.cancel:
			fmt.Fprintf(w, "event: cancel\ndata: {\"run_id\":%d}\n\n", f.run.ID)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	mux.HandleFunc("POST /api/runs/41/claim", authed(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.claims++
		if f.run.Status != "queued" {
			writeJSON(w, 409, map[string]any{"message": "a running run cannot be claimed"})
			return
		}
		f.run.Status = "running"
		writeJSON(w, 200, map[string]any{"run": f.run})
	}))
	state := func(w http.ResponseWriter) {
		writeJSON(w, 200, map[string]any{"run": map[string]any{"id": f.run.ID, "status": f.run.Status, "next_seq": f.run.NextSeq}})
	}
	notRunning := func(w http.ResponseWriter) bool {
		if f.run.Status != "running" {
			writeJSON(w, 409, map[string]any{"message": "a " + f.run.Status + " run cannot be reported"})
			return true
		}
		return false
	}
	mux.HandleFunc("POST /api/runs/41/events", authed(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Events []bakery.RunEvent `json:"events"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		if notRunning(w) {
			return
		}
		for _, e := range in.Events {
			if e.Seq < f.run.NextSeq {
				continue
			}
			if e.Seq != f.run.NextSeq {
				writeJSON(w, 422, map[string]any{"message": "gap", "expected_seq": f.run.NextSeq})
				return
			}
			f.events = append(f.events, e)
			f.run.NextSeq++
		}
		state(w)
	}))
	mux.HandleFunc("POST /api/runs/41/lease", authed(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if notRunning(w) {
			return
		}
		f.leases++
		state(w)
	}))
	mux.HandleFunc("POST /api/runs/41/finish", authed(func(w http.ResponseWriter, r *http.Request) {
		var in bakery.Finish
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		if notRunning(w) {
			return
		}
		f.finished = &in
		f.run.Status = in.Status
		state(w)
	}))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// startRunner runs a Runner with the stand-in against f until the test
// ends, and answers its `runs` updates.
func startRunner(t *testing.T, f *fakeBakery) <-chan RunUpdate {
	t.Helper()
	home := t.TempDir()
	s := store.New(filepath.Join(home, "bakeries.json"))
	if err := s.Put(store.Bakery{Address: f.srv.URL, DesktopID: 9, Key: "bky_desk_test"}); err != nil {
		t.Fatal(err)
	}
	// The stand-in exits 3 when it sees this: the Runner must remove it.
	t.Setenv("ANTHROPIC_API_KEY", "sk-must-not-reach-claude")
	t.Setenv("BAKERY_STANDIN_DELAY", "20ms")
	updates := make(chan RunUpdate, 1000)
	r := &Runner{Store: s, Claude: standin, Home: home, Logf: t.Logf, Events: func(name string, data any) {
		if u, ok := data.(RunUpdate); ok && name == "runs" {
			updates <- u
		}
	}}
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)
	t.Cleanup(func() { cancel(); r.Wait() })
	return updates
}

// until waits for an update with one of statuses.
func until(t *testing.T, updates <-chan RunUpdate, statuses ...string) RunUpdate {
	t.Helper()
	timeout := time.After(20 * time.Second)
	for {
		select {
		case u := <-updates:
			for _, s := range statuses {
				if u.Status == s {
					return u
				}
			}
		case <-timeout:
			t.Fatalf("no %v update", statuses)
		}
	}
}

func TestRunnerRunsAQueuedRun(t *testing.T) {
	f := newFakeBakery(t, "Fix the login page")
	updates := startRunner(t, f)
	u := until(t, updates, "succeeded", "failed", "stopped")
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Status != "succeeded" || u.RunID != 41 || u.AgentID != 3 || u.GuildID != 1 {
		t.Fatalf("update %+v; finish %+v", u, f.finished)
	}
	if f.claims != 1 {
		t.Fatalf("%d claims", f.claims)
	}
	var ks []string
	for i, e := range f.events {
		if e.Seq != int64(i+1) {
			t.Fatalf("event %d has seq %d", i, e.Seq)
		}
		ks = append(ks, e.Kind)
	}
	if got, want := strings.Join(ks, " "), "init assistant tool_call tool_result thinking assistant result"; got != want {
		t.Fatalf("events %q, want %q", got, want)
	}
	if f.finished.Status != "succeeded" || f.finished.Usage.Turns != 4 || f.finished.Usage.OutputTokens == 0 || *f.finished.ExitCode != 0 {
		t.Fatalf("finish %+v", f.finished)
	}
}

func TestRunnerStopsClaudeOnCancel(t *testing.T) {
	f := newFakeBakery(t, "Take your time [slow]")
	updates := startRunner(t, f)
	until(t, updates, "running")
	// Wait for a few of [slow]'s steps, then cancel as the Bakery does.
	deadline := time.Now().Add(10 * time.Second)
	for {
		f.mu.Lock()
		n := len(f.events)
		f.mu.Unlock()
		if n >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no events from the slow run")
		}
		time.Sleep(50 * time.Millisecond)
	}
	f.mu.Lock()
	f.run.Status = "cancelled"
	f.mu.Unlock()
	close(f.cancel)
	start := time.Now()
	u := until(t, updates, "cancelled", "stopped", "succeeded", "failed")
	if u.Status != "cancelled" {
		t.Fatalf("update %+v", u)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("stopping claude took %s", took)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.finished != nil {
		t.Fatalf("a cancelled run was finished: %+v", f.finished)
	}
	if out, _ := exec.Command("pgrep", "-f", standin).Output(); len(strings.TrimSpace(string(out))) > 0 {
		t.Fatalf("the stand-in still runs: %s", out)
	}
}

func TestRunnerLocalRunsReportsWhatIsRunning(t *testing.T) {
	f := newFakeBakery(t, "Take your time [slow]")
	home := t.TempDir()
	s := store.New(filepath.Join(home, "bakeries.json"))
	if err := s.Put(store.Bakery{Address: f.srv.URL, DesktopID: 9, Key: "bky_desk_test"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-must-not-reach-claude")
	t.Setenv("BAKERY_STANDIN_DELAY", "20ms")
	r := &Runner{Store: s, Claude: standin, Home: home, Logf: t.Logf}
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)
	t.Cleanup(func() { cancel(); r.Wait() })

	deadline := time.Now().Add(10 * time.Second)
	for {
		ls := r.LocalRuns()
		if len(ls) == 1 && len(ls[0].Events) > 0 {
			l := ls[0]
			if l.RunID != 41 || l.Agent.ID != 3 || l.Guild.ID != 1 || l.Status != "running" || l.Events[0].Seq != 1 {
				t.Fatalf("local run %+v", l)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("LocalRuns never showed the running run: %+v", ls)
		}
		time.Sleep(50 * time.Millisecond)
	}

	f.mu.Lock()
	f.run.Status = "cancelled"
	f.mu.Unlock()
	close(f.cancel)
	deadline = time.Now().Add(10 * time.Second)
	for len(r.LocalRuns()) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("LocalRuns still reports a stopped run: %+v", r.LocalRuns())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRunnerFailsARunWithoutClaude(t *testing.T) {
	f := newFakeBakery(t, "Anything")
	home := t.TempDir()
	s := store.New(filepath.Join(home, "bakeries.json"))
	_ = s.Put(store.Bakery{Address: f.srv.URL, Key: "bky_desk_test"})
	done := make(chan RunUpdate, 10)
	r := &Runner{Store: s, Claude: filepath.Join(home, "no-claude-here"), Home: home, Logf: t.Logf, Events: func(_ string, data any) {
		if u := data.(RunUpdate); u.Status != "running" {
			done <- u
		}
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); r.Wait() }()
	r.Start(ctx)
	if u := until(t, done, "failed", "succeeded", "stopped"); u.Status != "failed" {
		t.Fatalf("update %+v", u)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.Contains(f.finished.Error, "claude was not found") {
		t.Fatalf("finish %+v", f.finished)
	}
}

func TestRunnerTellsWhenSignedOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthenticated."})
	}))
	t.Cleanup(srv.Close)
	home := t.TempDir()
	s := store.New(filepath.Join(home, "bakeries.json"))
	if err := s.Put(store.Bakery{Address: srv.URL, DesktopID: 9, Key: "bky_desk_test"}); err != nil {
		t.Fatal(err)
	}
	told := make(chan struct{}, 10)
	r := &Runner{Store: s, Claude: standin, Home: home, Logf: t.Logf, Events: func(name string, _ any) {
		if name == "bakeries" {
			told <- struct{}{}
		}
	}}
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)
	t.Cleanup(func() { cancel(); r.Wait() })
	select {
	case <-told:
	case <-time.After(10 * time.Second):
		t.Fatal("no bakeries event")
	}
	f, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := f.Find(srv.URL); !b.SignedOut {
		t.Fatalf("bakery %+v not signed out", b)
	}
}
