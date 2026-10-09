package runner

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/jevido/bakery/apps/desktop/mcp"
	"github.com/jevido/bakery/apps/desktop/store"
)

// standin is the claude stand-in, built once for every test.
var standin string

func TestMain(m *testing.M) {
	// The Runner names its own executable as the bakery MCP server; in
	// these tests that is this test binary, started with `mcp`.
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		cfg, err := mcp.ConfigFromEnv(os.Getenv)
		if err == nil {
			err = mcp.Run(context.Background(), cfg, "test")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
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

func TestRunEnvGivesTheRunAndNoAPIKey(t *testing.T) {
	run := bakery.DesktopRun{ID: 41, Guild: bakery.Named{ID: 1}, Agent: bakery.RunAgent{ID: 3}, RunKey: "bky_run_x",
		Issue: &bakery.RunIssue{ID: 12}, WakeReason: "issue_assigned"}
	got := strings.Join(runEnv([]string{"PATH=/bin", "ANTHROPIC_API_KEY=sk-x", "BAKERY_API_KEY=old"}, "https://bakery.test", run, ""), ",")
	want := "PATH=/bin,BAKERY_API_KEY=old,BAKERY_AGENT_ID=3,BAKERY_API_KEY=bky_run_x,BAKERY_API_URL=https://bakery.test," +
		"BAKERY_GUILD_ID=1,BAKERY_ISSUE_ID=12,BAKERY_RUN_ID=41,BAKERY_WAKE_REASON=issue_assigned"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestPrepareWritesTheSkillAndTheMCPConfig(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{Self: "/opt/bakery-desktop"}
	run := bakery.DesktopRun{ID: 41, Guild: bakery.Named{ID: 1}, Agent: bakery.RunAgent{ID: 3}, RunKey: "bky_run_x"}
	args, scratch, err := r.prepare(dir, "https://bakery.test", run)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, ".bakery")
	config := filepath.Join(root, "mcp.json")
	if scratch != root {
		t.Fatalf("scratch %s", scratch)
	}
	if got, want := strings.Join(args[len(claudeArgs):], " "), "--mcp-config "+config+" --strict-mcp-config --add-dir "+root+" --allowedTools mcp__bakery"; got != want {
		t.Fatalf("args %q, want %q", got, want)
	}
	run.Workspace = &bakery.RunWorkspace{Repository: "https://git.test/guild/web.git", BaseBranch: "main", Branch: "bakery/def-12"}
	if args, _, err := r.prepare(t.TempDir(), "https://bakery.test", run); err != nil || args[len(args)-1] != "mcp__bakery,Bash(git:*)" {
		t.Fatalf("a Workspace Run's allowed tools %q: %v", args[len(args)-1], err)
	}
	if b, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "bakery", "SKILL.md")); err != nil || !strings.HasPrefix(string(b), "---\nname: bakery\n") {
		t.Fatalf("skill %q: %v", b, err)
	}
	st, err := os.Stat(config)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mcp.json %v: %v", st, err)
	}
	var c struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	b, _ := os.ReadFile(config)
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	s := c.MCPServers["bakery"]
	if len(c.MCPServers) != 1 || s.Command != "/opt/bakery-desktop" || strings.Join(s.Args, " ") != "mcp" ||
		s.Env["BAKERY_API_KEY"] != "bky_run_x" || s.Env["BAKERY_API_URL"] != "https://bakery.test" || s.Env["BAKERY_RUN_ID"] != "41" {
		t.Fatalf("mcp.json %s", b)
	}
}

func TestPrepareWritesTheAgentsSkills(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{Self: "/opt/bakery-desktop"}
	run := bakery.DesktopRun{ID: 41, Guild: bakery.Named{ID: 1}, Agent: bakery.RunAgent{ID: 3}, RunKey: "bky_run_x",
		Skills: []bakery.RunSkill{
			{Slug: "release-notes", Files: []bakery.RunSkillFile{
				{Path: "SKILL.md", Content: "---\nname: Release notes\n---\n", Encoding: "utf8"},
				{Path: "templates/notes.md", Content: "# Notes\n", Encoding: "utf8"},
			}},
			{Slug: "lint", Files: []bakery.RunSkillFile{
				{Path: "SKILL.md", Content: "---\nname: Lint\n---\n", Encoding: "utf8"},
				{Path: "bin/run.sh", Content: base64.StdEncoding.EncodeToString([]byte("#!/bin/sh\n")), Encoding: "base64", Executable: true},
			}},
		}}
	if _, _, err := r.prepare(dir, "https://bakery.test", run); err != nil {
		t.Fatal(err)
	}
	skills := filepath.Join(dir, ".bakery", ".claude", "skills")
	for path, want := range map[string]struct {
		content string
		mode    os.FileMode
	}{
		"release-notes/SKILL.md":           {"---\nname: Release notes\n---\n", 0o600},
		"release-notes/templates/notes.md": {"# Notes\n", 0o600},
		"lint/bin/run.sh":                  {"#!/bin/sh\n", 0o700},
		"bakery/SKILL.md":                  {string(skill), 0o600},
	} {
		b, err := os.ReadFile(filepath.Join(skills, path))
		st, _ := os.Stat(filepath.Join(skills, path))
		if err != nil || string(b) != want.content || st.Mode().Perm() != want.mode {
			t.Fatalf("%s: %q %v: %v", path, b, st, err)
		}
	}
	if st, err := os.Stat(filepath.Join(skills, "release-notes", "templates")); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("templates directory %v: %v", st, err)
	}

	for _, bad := range []bakery.RunSkill{
		{Slug: "evil", Files: []bakery.RunSkillFile{{Path: "../evil", Content: "x"}}},
		{Slug: "evil", Files: []bakery.RunSkillFile{{Path: "a/../../evil", Content: "x"}}},
		{Slug: "evil", Files: []bakery.RunSkillFile{{Path: "/etc/evil", Content: "x"}}},
		{Slug: "bakery", Files: []bakery.RunSkillFile{{Path: "SKILL.md", Content: "x"}}},
		{Slug: "../up", Files: []bakery.RunSkillFile{{Path: "SKILL.md", Content: "x"}}},
	} {
		dir := t.TempDir()
		run.Skills = []bakery.RunSkill{bad}
		if _, _, err := r.prepare(dir, "https://bakery.test", run); err == nil {
			t.Fatalf("%s %s was written", bad.Slug, bad.Files[0].Path)
		}
		if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
			t.Fatalf("%s %s left the skill's directory", bad.Slug, bad.Files[0].Path)
		}
	}
}

func TestSkillSaysNothingOfPaperclip(t *testing.T) {
	for _, word := range []string{"Paperclip", "paperclip", "PAPERCLIP", "company"} {
		if strings.Contains(string(skill), word) {
			t.Fatalf("the skill says %q", word)
		}
	}
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

func TestTranscriptOfALimitedRun(t *testing.T) {
	t.Setenv("BAKERY_STANDIN_STATE", t.TempDir())
	t.Setenv("BAKERY_STANDIN_LIMIT_RESET", "10m")
	stdout, _, code := standinLines(t, "DEF-12: Do it [limit]")
	tr := &Transcript{}
	var events []Event
	for _, l := range stdout {
		events = append(events, tr.Stdout(l)...)
	}
	// The rate_limit_event shows nowhere.
	if got := kinds(events); got != "init assistant result" {
		t.Fatalf("kinds %q", got)
	}
	f := tr.Finish(code)
	if f.Status != "limited" || f.LimitResetsAt == nil || *f.ExitCode != 1 || !strings.HasPrefix(f.Error, "You've hit your limit") || f.Usage.Turns != 1 {
		t.Fatalf("finish %+v", f)
	}
	if !f.LimitResetsAt.Equal(*tr.LimitResetsAt) {
		t.Fatalf("reset %v, the event said %v", f.LimitResetsAt, tr.LimitResetsAt)
	}
	if d := time.Until(*f.LimitResetsAt); d < 9*time.Minute || d > 10*time.Minute {
		t.Fatalf("reset %v ahead, want about 10m", d)
	}
}

func TestTranscriptReadsTheLimitFromItsWording(t *testing.T) {
	now := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	result := func(text string) string {
		b, _ := json.Marshal(map[string]any{"type": "result", "subtype": "success", "is_error": true, "result": text})
		return string(b)
	}
	cases := []struct {
		name string
		feed func(*Transcript)
		want time.Time
	}{
		{"result text", func(tr *Transcript) { tr.Stdout(result("You've hit your limit · resets 2:30am (UTC)")) },
			time.Date(2026, 10, 10, 2, 30, 0, 0, time.UTC)},
		{"later today in a named zone", func(tr *Transcript) {
			tr.Stdout(result("Claude usage limit reached. Your limit resets 7pm (Europe/Amsterdam)."))
		}, time.Date(2026, 10, 9, 17, 0, 0, 0, time.UTC)},
		{"assistant text and exit without result", func(tr *Transcript) {
			b, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "You’ve hit your session limit · resets 3:15pm (UTC)"}}}})
			tr.Stdout(string(b))
		}, time.Date(2026, 10, 9, 15, 15, 0, 0, time.UTC)},
		{"stderr", func(tr *Transcript) { tr.Stderr("Error: 5-hour limit reached, resets 11pm (UTC)") },
			time.Date(2026, 10, 9, 23, 0, 0, 0, time.UTC)},
		{"no readable time", func(tr *Transcript) { tr.Stdout(result("Claude usage limit reached.")) },
			now.Add(time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := &Transcript{now: clock}
			c.feed(tr)
			f := tr.Finish(1)
			if f.Status != "limited" || f.LimitResetsAt == nil || !f.LimitResetsAt.Equal(c.want) {
				t.Fatalf("finish %+v, reset %v, want %v", f, f.LimitResetsAt, c.want)
			}
		})
	}
	// A rate_limit_event that is not rejected is no limit.
	tr := &Transcript{now: clock}
	tr.Stdout(`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":1760000000,"rateLimitType":"five_hour"}}`)
	tr.Stdout(result("Something else went wrong."))
	if f := tr.Finish(1); f.Status != "failed" || f.LimitResetsAt != nil {
		t.Fatalf("finish %+v", f)
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
	// calls are the requests made with the Run key, as "METHOD path body".
	calls []string
	// onCall, when set, runs on each request made with the Run key.
	onCall func()
	// push sends the list on the stream again, as a new queued Run does.
	push chan struct{}
	// refuseUntil, while ahead, refuses claims as a Desktop at its
	// Subscription limit.
	refuseUntil time.Time
	// claimedAt is when the last claim was taken.
	claimedAt time.Time
}

func newFakeBakery(t *testing.T, prompt string) *fakeBakery {
	f := &fakeBakery{t: t, cancel: make(chan struct{}), push: make(chan struct{}, 1),
		run: bakery.DesktopRun{ID: 41, Status: "queued", Guild: bakery.Named{ID: 1, Name: "Bakers"}, Agent: bakery.RunAgent{ID: 3, Name: "Ada"}, Prompt: prompt, NextSeq: 1,
			Issue: &bakery.RunIssue{ID: 12, Identifier: "DEF-12", Title: "Fix it"}, WakeReason: "issue_assigned"}}
	mux := http.NewServeMux()
	// What the bakery MCP server sends with the Run key, which only
	// the claimed Run's claude has.
	mux.HandleFunc("/api/issues/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bky_run_test" || r.Header.Get("Bakery-Guild") != "1" {
			writeJSON(w, 401, map[string]any{"message": "Unauthenticated."})
			return
		}
		var body strings.Builder
		_, _ = io.Copy(&body, r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(body.String()))
		onCall := f.onCall
		f.mu.Unlock()
		if onCall != nil {
			onCall()
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
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
		for {
			select {
			case <-r.Context().Done():
				return
			case <-f.push:
				f.mu.Lock()
				b, _ := json.Marshal(map[string]any{"runs": list()})
				f.mu.Unlock()
				fmt.Fprintf(w, "event: runs\ndata: %s\n\n", b)
				w.(http.Flusher).Flush()
			case <-f.cancel:
				fmt.Fprintf(w, "event: cancel\ndata: {\"run_id\":%d}\n\n", f.run.ID)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				return
			}
		}
	}))
	mux.HandleFunc("POST /api/runs/41/claim", authed(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.claims++
		if time.Now().Before(f.refuseUntil) {
			writeJSON(w, 409, map[string]any{"message": "subscription limit until " + f.refuseUntil.UTC().Format(time.RFC3339), "resets_at": f.refuseUntil.UTC()})
			return
		}
		if f.run.Status != "queued" {
			writeJSON(w, 409, map[string]any{"message": "a running run cannot be claimed"})
			return
		}
		f.run.Status = "running"
		f.claimedAt = time.Now()
		claimed := f.run
		claimed.RunKey = "bky_run_test"
		writeJSON(w, 200, map[string]any{"run": claimed})
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
	return startRunnerIn(t, f, t.TempDir())
}

func startRunnerIn(t *testing.T, f *fakeBakery, home string) <-chan RunUpdate {
	t.Helper()
	updates, _, _ := startRunners(t, home, f)
	return updates
}

// startRunners runs one Runner against every fake in fs, with home's
// bakeries.json as it is plus them, and answers its `runs` and `limit`
// updates.
func startRunners(t *testing.T, home string, fs ...*fakeBakery) (<-chan RunUpdate, <-chan LimitUpdate, *Runner) {
	t.Helper()
	s := store.New(filepath.Join(home, "bakeries.json"))
	for _, f := range fs {
		if err := s.Put(store.Bakery{Address: f.srv.URL, DesktopID: 9, Key: "bky_desk_test"}); err != nil {
			t.Fatal(err)
		}
	}
	// The stand-in exits 3 when it sees this: the Runner must remove it.
	t.Setenv("ANTHROPIC_API_KEY", "sk-must-not-reach-claude")
	t.Setenv("BAKERY_STANDIN_DELAY", "20ms")
	updates := make(chan RunUpdate, 1000)
	limits := make(chan LimitUpdate, 100)
	r := &Runner{Store: s, Claude: standin, Home: home, Logf: t.Logf, Events: func(name string, data any) {
		if u, ok := data.(RunUpdate); ok && name == "runs" {
			updates <- u
		}
		if u, ok := data.(LimitUpdate); ok && name == "limit" {
			limits <- u
		}
	}}
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)
	t.Cleanup(func() { cancel(); r.Wait() })
	return updates, limits, r
}

// queue puts the fake's Run back in the queue and says so on its stream.
func (f *fakeBakery) queue() {
	f.mu.Lock()
	f.run.Status = "queued"
	f.mu.Unlock()
	select {
	case f.push <- struct{}{}:
	default:
	}
}

func (f *fakeBakery) state() (status string, claims int, claimedAt time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.run.Status, f.claims, f.claimedAt
}

// untilFrom waits for an update from the Bakery at address with one of
// statuses.
func untilFrom(t *testing.T, updates <-chan RunUpdate, address string, statuses ...string) RunUpdate {
	t.Helper()
	for {
		u := until(t, updates, statuses...)
		if u.Address == address {
			return u
		}
	}
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

func TestRunnerGivesClaudeTheBakerysTools(t *testing.T) {
	f := newFakeBakery(t, `Fix it [mcp bakeryCheckoutIssue {"issueId":"DEF-12"}] [mcp bakeryAddComment {"issueId":"DEF-12","body":"hi"}]`)
	home := t.TempDir()
	skillFile := filepath.Join(home, "runs", "41", ".bakery", ".claude", "skills", "bakery", "SKILL.md")
	skillThere := 0
	f.onCall = func() {
		if _, err := os.Stat(skillFile); err == nil {
			skillThere++
		}
	}
	updates := startRunnerIn(t, f, home)
	u := until(t, updates, "succeeded", "failed", "stopped")
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Status != "succeeded" {
		t.Fatalf("update %+v; finish %+v; events %+v", u, f.finished, f.events)
	}
	want := []string{
		`POST /api/issues/DEF-12/checkout {"expected_statuses":["todo","backlog","blocked"]}`,
		`POST /api/issues/DEF-12/comments {"body":"hi"}`,
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls\n%s\nwant\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
	if skillThere != 2 {
		t.Fatalf("the skill was there for %d of 2 calls", skillThere)
	}
	if _, err := os.Stat(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(skillFile))))); !os.IsNotExist(err) {
		t.Fatalf("the Run's .bakery is still there: %v", err)
	}
	var ks []string
	var tools []string
	for _, e := range f.events {
		ks = append(ks, e.Kind)
		if e.Kind == "tool_call" {
			var call struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(e.Payload, &call)
			tools = append(tools, call.Name)
		}
	}
	if got, want := strings.Join(ks, " "), "init tool_call tool_result tool_call tool_result assistant result"; got != want {
		t.Fatalf("events %q, want %q", got, want)
	}
	if got := strings.Join(tools, " "); got != "mcp__bakery__bakeryCheckoutIssue mcp__bakery__bakeryAddComment" {
		t.Fatalf("tool calls %q", got)
	}
}

func TestRunnerStartsClaudeInTheIssuesWorktree(t *testing.T) {
	_, url := originRepo(t)
	f := newFakeBakery(t, "Where are you [env]")
	f.run.Workspace = &bakery.RunWorkspace{Application: bakery.Named{ID: 5, Name: "web"}, Repository: url, BaseBranch: "main", Branch: "bakery/def-12"}
	home := t.TempDir()
	updates := startRunnerIn(t, f, home)
	u := until(t, updates, "succeeded", "failed", "stopped")
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Status != "succeeded" {
		t.Fatalf("update %+v; finish %+v", u, f.finished)
	}
	worktree := filepath.Join(home, "worktrees", shortHash(f.srv.URL, 8), "12")
	host := strings.Split(strings.TrimPrefix(f.srv.URL, "http://"), ":")[0]
	var said string
	for _, e := range f.events {
		if e.Kind == "assistant" {
			said += string(e.Payload)
		}
	}
	for _, want := range []string{
		"cwd=" + worktree, "GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=agent-3@" + host,
		"BAKERY_WORKTREE=" + worktree, "BAKERY_BRANCH=bakery/def-12", "BAKERY_BASE_BRANCH=main",
	} {
		if !strings.Contains(said, want) {
			t.Fatalf("claude did not say %q:\n%s", want, said)
		}
	}
	if got := gitIn(t, worktree, "branch", "--show-current"); got != "bakery/def-12" {
		t.Fatalf("the Worktree is on %q", got)
	}
}

func TestRunnerPushesTheAgentBranchAndOpensThePullRequest(t *testing.T) {
	origin, url := originRepo(t)
	f := newFakeBakery(t, `Fix it [git commit index.html hello] [git push] [mcp bakeryOpenPullRequest {"issueId":"DEF-12"}]`)
	f.run.Workspace = &bakery.RunWorkspace{Application: bakery.Named{ID: 5, Name: "web"}, Repository: url, BaseBranch: "main", Branch: "bakery/def-12"}
	updates := startRunner(t, f)
	u := until(t, updates, "succeeded", "failed", "stopped")
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Status != "succeeded" {
		t.Fatalf("update %+v; finish %+v", u, f.finished)
	}
	if got := gitIn(t, origin, "show", "bakery/def-12:index.html"); got != "hello" {
		t.Fatalf("pushed index.html %q", got)
	}
	if got := gitIn(t, origin, "log", "-1", "--format=%an", "bakery/def-12"); got != "Ada" {
		t.Fatalf("the commit's author is %q, not the Agent", got)
	}
	if got := gitIn(t, origin, "rev-parse", "main"); got != gitIn(t, origin, "rev-parse", "bakery/def-12~1") {
		t.Fatal("main moved, or the Agent branch is not one commit on it")
	}
	if want := `POST /api/issues/DEF-12/pull-requests {"body":"","title":""}`; strings.Join(f.calls, "\n") != want {
		t.Fatalf("calls %q, want %q", f.calls, want)
	}
}

func TestRunnerFailsARunWhoseRepositoryIsUnreachable(t *testing.T) {
	isolateGit(t)
	f := newFakeBakery(t, "Fix it")
	f.run.Workspace = &bakery.RunWorkspace{Repository: "file://" + filepath.Join(t.TempDir(), "missing.git"), BaseBranch: "main", Branch: "bakery/def-12"}
	updates := startRunner(t, f)
	u := until(t, updates, "succeeded", "failed", "stopped")
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Status != "failed" || !strings.HasPrefix(f.finished.Error, "preparing the worktree: git clone: ") {
		t.Fatalf("update %+v; finish %+v", u, f.finished)
	}
}

func TestRunnerWaitsOutTheLimitOnEveryBakery(t *testing.T) {
	t.Setenv("BAKERY_STANDIN_STATE", t.TempDir())
	t.Setenv("BAKERY_STANDIN_LIMIT_RESET", "3s")
	a := newFakeBakery(t, "DEF-12: Fix it [limit]")
	b := newFakeBakery(t, "DEF-20: Other")
	b.run.Status = "held"
	home := t.TempDir()
	updates, limits, r := startRunners(t, home, a, b)

	untilFrom(t, updates, a.srv.URL, "limited", "failed", "stopped")
	a.mu.Lock()
	fin := *a.finished
	a.mu.Unlock()
	if fin.Status != "limited" || fin.LimitResetsAt == nil {
		t.Fatalf("finish %+v", fin)
	}
	resets := *fin.LimitResetsAt
	if l := <-limits; l.Until == nil || !l.Until.Equal(resets) {
		t.Fatalf("limit event %+v, want %v", l, resets)
	}
	if !r.Limit().Equal(resets) {
		t.Fatalf("Limit() %v, want %v", r.Limit(), resets)
	}
	if f, _ := store.New(filepath.Join(home, "bakeries.json")).Load(); f.LimitedUntil == nil || !f.LimitedUntil.Equal(resets) {
		t.Fatalf("bakeries.json limited_until %v", f.LimitedUntil)
	}

	// Both Bakeries have a queued Run now; neither is claimed before the reset.
	a.queue()
	b.queue()
	time.Sleep(500 * time.Millisecond)
	if _, n, _ := a.state(); n != 1 {
		t.Fatalf("%d claims on the limited Bakery before the reset", n)
	}
	if _, n, _ := b.state(); n != 0 {
		t.Fatalf("%d claims on the other Bakery before the reset", n)
	}

	untilFrom(t, updates, a.srv.URL, "succeeded")
	untilFrom(t, updates, b.srv.URL, "succeeded")
	for _, f := range []*fakeBakery{a, b} {
		if _, _, at := f.state(); at.Before(resets) {
			t.Fatalf("claimed at %v, before the reset %v", at, resets)
		}
	}
	if l := <-limits; l.Until != nil {
		t.Fatalf("reset event %+v", l)
	}
	if f, _ := store.New(filepath.Join(home, "bakeries.json")).Load(); f.LimitedUntil != nil {
		t.Fatalf("bakeries.json still limited until %v", f.LimitedUntil)
	}
}

func TestRunnerRemembersTheLimitAcrossARestart(t *testing.T) {
	home := t.TempDir()
	resets := time.Now().Add(1500 * time.Millisecond).UTC()
	if _, err := store.New(filepath.Join(home, "bakeries.json")).Update(func(f *store.File) error {
		f.LimitedUntil = &resets
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f := newFakeBakery(t, "DEF-12: Fix it")
	updates, _, _ := startRunners(t, home, f)
	time.Sleep(500 * time.Millisecond)
	if _, n, _ := f.state(); n != 0 {
		t.Fatalf("%d claims before the reset", n)
	}
	until(t, updates, "succeeded")
	if _, _, at := f.state(); at.Before(resets) {
		t.Fatalf("claimed at %v, before the reset %v", at, resets)
	}
}

func TestRunnerHoldsOffWhenTheBakeryRefusesAtTheLimit(t *testing.T) {
	f := newFakeBakery(t, "DEF-12: Fix it")
	resets := time.Now().Add(1500 * time.Millisecond).Truncate(time.Second).Add(time.Second)
	f.refuseUntil = resets
	updates, limits, _ := startRunners(t, t.TempDir(), f)
	if l := <-limits; l.Until == nil || !l.Until.Equal(resets) {
		t.Fatalf("limit event %+v, want %v", l, resets)
	}
	f.queue()
	time.Sleep(300 * time.Millisecond)
	if _, n, _ := f.state(); n != 1 {
		t.Fatalf("%d claims before the reset, want the one refused", n)
	}
	until(t, updates, "succeeded")
	if _, n, at := f.state(); n != 2 || at.Before(resets) {
		t.Fatalf("%d claims, the last at %v (reset %v)", n, at, resets)
	}
}
