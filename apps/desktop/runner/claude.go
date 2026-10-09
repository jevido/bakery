package runner

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
)

// How the Runner reports: a batch every 250 ms or at 50 events, a Lease
// keep-alive after 30 s with nothing sent (the Bakery's Lease is 90 s),
// and SIGKILL 5 s after a cancel's SIGTERM.
var (
	flushEvery = 250 * time.Millisecond
	flushAt    = 50
	leaseEvery = 30 * time.Second
	killAfter  = 5 * time.Second
	retryEvery = 2 * time.Second
	retryFor   = 2 * time.Minute
)

// The most events in one report (the Bakery takes 500).
const maxBatch = 500

// claudeArgs are claude's arguments, the prompt going on stdin, as
// Paperclip's claude adapter passes them.
var claudeArgs = []string{"--print", "-", "--output-format", "stream-json", "--verbose", "--permission-mode", "acceptEdits"}

// strippedEnv are removed from claude's environment so it runs on the
// person's own login, never on an API key.
var strippedEnv = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"}

// claudeEnv is env without the variables that would make claude use an
// API key instead of the person's subscription.
func claudeEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		stripped := false
		for _, s := range strippedEnv {
			if strings.EqualFold(name, s) {
				stripped = true
			}
		}
		if !stripped {
			out = append(out, kv)
		}
	}
	return out
}

// skill is The Bakery skill, written for each Run where claude finds it.
//
//go:embed skill/bakery/SKILL.md
var skill []byte

// runArgs are claudeArgs plus what gives claude The Bakery: the MCP server
// in mcpConfig and nothing else (--strict-mcp-config, so the person's own
// MCP servers stay out of an Agent's Run), the skill under addDir (claude
// loads skills from .claude/skills in a directory added with --add-dir,
// as Paperclip's claude adapter relies on), and the server's tools allowed
// without asking. A Run with a Workspace may also run git without asking,
// and nothing else through Bash: it commits and pushes in its Worktree, and
// --print has nobody to ask, so a git call would only be refused.
func runArgs(mcpConfig, addDir string, workspace bool) []string {
	tools := "mcp__bakery"
	if workspace {
		tools += ",Bash(git:*)"
	}
	return append(append([]string{}, claudeArgs...),
		"--mcp-config", mcpConfig, "--strict-mcp-config", "--add-dir", addDir, "--allowedTools", tools)
}

// mcpEnv is what the Bakery's MCP server needs to act for run on the
// Bakery at address (mcp.ConfigFromEnv reads it).
func mcpEnv(address string, run bakery.DesktopRun) map[string]string {
	return map[string]string{
		"BAKERY_API_URL":  address,
		"BAKERY_API_KEY":  run.RunKey,
		"BAKERY_GUILD_ID": strconv.FormatUint(run.Guild.ID, 10),
		"BAKERY_AGENT_ID": strconv.FormatUint(run.Agent.ID, 10),
		"BAKERY_RUN_ID":   strconv.FormatUint(run.ID, 10),
	}
}

// runEnv is claude's environment for run: env without API keys, plus the
// Run's Bakery, key, Guild, Agent and Run, and why it was woken (Paperclip's
// PAPERCLIP_* wake variables), so the skill's curl fallback works too. A Run
// with a Workspace also gets its Worktree and branches, and the Agent as
// git's author; the committer stays the person's own git config.
func runEnv(env []string, address string, run bakery.DesktopRun, worktree string) []string {
	out := claudeEnv(env)
	vars := mcpEnv(address, run)
	if run.Issue != nil {
		vars["BAKERY_ISSUE_ID"] = strconv.FormatUint(run.Issue.ID, 10)
	}
	if run.WakeReason != "" {
		vars["BAKERY_WAKE_REASON"] = run.WakeReason
	}
	if ws := run.Workspace; ws != nil && worktree != "" {
		vars["BAKERY_WORKTREE"] = worktree
		vars["BAKERY_BRANCH"] = ws.Branch
		vars["BAKERY_BASE_BRANCH"] = ws.BaseBranch
		vars["GIT_AUTHOR_NAME"] = run.Agent.Name
		vars["GIT_AUTHOR_EMAIL"] = "agent-" + strconv.FormatUint(run.Agent.ID, 10) + "@" + hostOf(address)
		// A push without credentials fails instead of waiting on a prompt
		// nobody sees.
		vars["GIT_TERMINAL_PROMPT"] = "0"
	}
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	// os/exec keeps the last of a repeated name, so these win over any
	// BAKERY_* the desktop itself was started with.
	for _, name := range names {
		out = append(out, name+"="+vars[name])
	}
	return out
}

// hostOf is address's host without its port: the Agent's git email domain.
func hostOf(address string) string {
	if u, err := url.Parse(address); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "bakery.invalid"
}

// prepare writes what claude needs for run under dir/.bakery: the skill at
// .claude/skills/bakery/SKILL.md and mcp.json naming this executable's
// `mcp` command as the bakery server. It answers claude's arguments and the
// directory to remove when the Run ends; mcp.json holds the Run key, so it
// is the person's alone (0600).
func (r *Runner) prepare(dir, address string, run bakery.DesktopRun) ([]string, string, error) {
	self, err := r.self()
	if err != nil {
		return nil, "", fmt.Errorf("finding the desktop app's own binary: %w", err)
	}
	root := filepath.Join(dir, ".bakery")
	skillDir := filepath.Join(root, ".claude", "skills", "bakery")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skill, 0o600); err != nil {
		return nil, root, err
	}
	config, err := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{
		"bakery": map[string]any{"type": "stdio", "command": self, "args": []string{"mcp"}, "env": mcpEnv(address, run)},
	}}, "", "  ")
	if err != nil {
		return nil, root, err
	}
	mcpConfig := filepath.Join(root, "mcp.json")
	if err := os.WriteFile(mcpConfig, config, 0o600); err != nil {
		return nil, root, err
	}
	return runArgs(mcpConfig, root, run.Workspace != nil), root, nil
}

// self is the binary that serves `mcp`: Self, else this executable.
func (r *Runner) self() (string, error) {
	if r.Self != "" {
		return r.Self, nil
	}
	return os.Executable()
}

// claude is the binary to start: Claude, else BAKERY_CLAUDE, else
// `claude` on PATH.
func (r *Runner) claude() (string, error) {
	name := r.Claude
	if name == "" {
		name = strings.TrimSpace(os.Getenv("BAKERY_CLAUDE"))
	}
	if name == "" {
		path, err := exec.LookPath("claude")
		if err != nil {
			return "", errors.New("claude was not found on PATH")
		}
		return path, nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("claude was not found at %s", name)
	}
	return path, nil
}

func (r *Runner) home() string {
	if r.Home != "" {
		return r.Home
	}
	return os.TempDir()
}

// outLine is one line claude printed, on stdout or stderr.
type outLine struct {
	stderr bool
	text   string
}

// execute runs claude for the claimed Run, reports what it prints and
// finishes the Run, unless it was cancelled, the Bakery stopped having it
// running, or ctx ended (the Runner stopping), when claude is stopped and
// the Run is left as the Bakery has it.
func (r *Runner) execute(ctx context.Context, c *bakery.Client, run bakery.DesktopRun, e *execution) {
	rep := &reporter{c: c, id: run.ID, next: max(run.NextSeq, 1), lastSent: time.Now()}
	r.emit(c, run, "running", 0)
	fail := func(msg string) {
		r.finish(ctx, c, run, rep, bakery.Finish{Status: "failed", Error: msg})
	}
	dir := filepath.Join(r.home(), "runs", strconv.FormatUint(run.ID, 10))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fail("preparing the run's directory: " + err.Error())
		return
	}
	bin, err := r.claude()
	if err != nil {
		fail(err.Error())
		return
	}
	args, scratch, err := r.prepare(dir, c.Address, run)
	if scratch != "" {
		defer os.RemoveAll(scratch)
	}
	if err != nil {
		fail("preparing the run's directory: " + err.Error())
		return
	}
	// claude works in the Issue's Worktree when the Run has a Workspace;
	// the scratch .bakery stays in the Run's own directory either way.
	work := dir
	if ws := run.Workspace; ws != nil && run.Issue != nil {
		if work, err = r.trees().prepare(ctx, c.Address, run.Issue.ID, *ws); err != nil {
			fail("preparing the worktree: " + err.Error())
			return
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = work
	worktree := ""
	if work != dir {
		worktree = work
	}
	cmd.Env = runEnv(os.Environ(), c.Address, run, worktree)
	cmd.Stdin = strings.NewReader(run.Prompt)
	ownGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fail(err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fail(err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		fail("starting claude: " + err.Error())
		return
	}

	out := make(chan outLine, 256)
	var readers sync.WaitGroup
	readers.Add(2)
	go read(stdout, false, out, &readers)
	go read(stderr, true, out, &readers)
	go func() { readers.Wait(); close(out) }()

	t := &Transcript{}
	ticker := time.NewTicker(flushEvery)
	defer ticker.Stop()
	cancelled := e.cancelled
	var stopping *time.Timer
	stop := func() {
		if stopping == nil {
			terminate(cmd)
			stopping = time.AfterFunc(killAfter, func() { kill(cmd) })
		}
	}
	// stopped is why claude was stopped: cancelled, or stopped when the
	// Bakery no longer has the Run running here.
	stopped, interrupted := "", false
	for out != nil {
		select {
		case l, ok := <-out:
			if !ok {
				out = nil
				continue
			}
			if l.stderr {
				e.record(toLocal(rep.add(t.Stderr(l.text))))
			} else {
				e.record(toLocal(rep.add(t.Stdout(l.text))))
			}
			if len(rep.pending) >= flushAt {
				r.report(ctx, c, run, rep)
			}
		case <-ticker.C:
			r.report(ctx, c, run, rep)
		case <-cancelled:
			cancelled, stopped = nil, "cancelled"
			r.logf("runner: run %d cancelled on %s; stopping claude", run.ID, c.Address)
			stop()
		case <-ctx.Done():
			// The Runner is stopping: claude goes now, and the Run is
			// left for the Lease sweep to queue again.
			kill(cmd)
			interrupted = true
			ctx = context.WithoutCancel(ctx)
		}
		if rep.gone && stopped == "" {
			stopped = "stopped"
			r.logf("runner: run %d is no longer running on %s; stopping claude", run.ID, c.Address)
			stop()
		}
	}
	waitErr := cmd.Wait()
	if stopping != nil {
		stopping.Stop()
	}
	// Whatever claude's children left behind goes too.
	kill(cmd)
	// The Run key goes with claude, before anyone hears that the Run ended.
	os.RemoveAll(scratch)
	if interrupted {
		return
	}
	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if waitErr != nil {
		exitCode = -1
	}
	if stopped != "" {
		r.drain(ctx, rep)
		r.emit(c, run, stopped, rep.next-1)
		r.logf("runner: run %d stopped on %s", run.ID, c.Address)
		return
	}
	r.finish(ctx, c, run, rep, t.Finish(exitCode))
}

func read(rd io.Reader, stderr bool, out chan<- outLine, wg *sync.WaitGroup) {
	defer wg.Done()
	s := bufio.NewScanner(rd)
	s.Buffer(make([]byte, 64<<10), 32<<20)
	for s.Scan() {
		out <- outLine{stderr: stderr, text: s.Text()}
	}
	// A line over the buffer ends the scan; drain the rest so claude is
	// never blocked on a full pipe.
	_, _ = io.Copy(io.Discard, rd)
}

// report sends what is pending, or keeps the Lease when nothing was sent
// for leaseEvery.
func (r *Runner) report(ctx context.Context, c *bakery.Client, run bakery.DesktopRun, rep *reporter) {
	if len(rep.pending) > 0 {
		if rep.flush(ctx) == nil {
			r.emit(c, run, "running", rep.next-1)
		}
		return
	}
	if time.Since(rep.lastSent) >= leaseEvery {
		rep.keep(ctx)
	}
}

// drain sends what is still pending, trying again for up to retryFor.
func (r *Runner) drain(ctx context.Context, rep *reporter) {
	deadline := time.Now().Add(retryFor)
	for len(rep.pending) > 0 && !rep.gone && time.Now().Before(deadline) {
		if rep.flush(ctx) == nil {
			continue
		}
		if rep.signedOut || !sleep(ctx, retryEvery) {
			return
		}
	}
}

// finish sends the last events and then the Run's end, trying again for
// up to retryFor while the Bakery cannot be reached.
func (r *Runner) finish(ctx context.Context, c *bakery.Client, run bakery.DesktopRun, rep *reporter, f bakery.Finish) {
	if f.Status == "limited" && f.LimitResetsAt != nil {
		r.limit(*f.LimitResetsAt)
	}
	r.drain(ctx, rep)
	if rep.gone {
		r.emit(c, run, "stopped", rep.next-1)
		return
	}
	deadline := time.Now().Add(retryFor)
	for {
		st, err := c.FinishRun(ctx, run.ID, f)
		if err == nil {
			r.emit(c, run, st.Status, rep.next-1)
			r.logf("runner: run %d %s on %s", run.ID, st.Status, c.Address)
			return
		}
		var be *bakery.Error
		if errors.Is(err, bakery.ErrSignedOut) || errors.Is(err, bakery.ErrNotFound) || errors.As(err, &be) || time.Now().After(deadline) {
			r.logf("runner: finishing run %d on %s: %v", run.ID, c.Address, err)
			r.emit(c, run, "stopped", rep.next-1)
			return
		}
		if !sleep(ctx, retryEvery) {
			return
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// reporter numbers a Run's events and sends them. A report that failed
// keeps its events for the next one; the Bakery ignores a seq it has.
type reporter struct {
	c        *bakery.Client
	id       uint64
	next     int64
	pending  []bakery.RunEvent
	lastSent time.Time
	// gone is set when the Bakery no longer has the Run running here
	// (cancelled, lost, or another Desktop's): nothing more is sent.
	gone      bool
	signedOut bool
}

// add numbers events and queues them to send, answering them numbered for
// LocalRuns to record.
func (p *reporter) add(events []Event) []bakery.RunEvent {
	if p.gone || len(events) == 0 {
		return nil
	}
	added := make([]bakery.RunEvent, 0, len(events))
	for _, e := range events {
		re := bakery.RunEvent{Seq: p.next, Kind: e.Kind, Payload: e.Payload}
		p.pending = append(p.pending, re)
		added = append(added, re)
		p.next++
	}
	return added
}

// toLocal turns numbered Run events into LocalEvents, stamped with now.
func toLocal(events []bakery.RunEvent) []LocalEvent {
	if len(events) == 0 {
		return nil
	}
	now := time.Now()
	out := make([]LocalEvent, len(events))
	for i, e := range events {
		out[i] = LocalEvent{Seq: e.Seq, Kind: e.Kind, Payload: e.Payload, CreatedAt: now}
	}
	return out
}

func (p *reporter) flush(ctx context.Context) error {
	if len(p.pending) == 0 || p.gone {
		return nil
	}
	batch := p.pending[:min(len(p.pending), maxBatch)]
	st, err := p.c.AppendRunEvents(ctx, p.id, batch)
	if err != nil {
		p.note(err)
		return err
	}
	kept := p.pending[:0]
	for _, e := range p.pending {
		if e.Seq >= st.NextSeq {
			kept = append(kept, e)
		}
	}
	p.pending, p.lastSent = kept, time.Now()
	return nil
}

func (p *reporter) keep(ctx context.Context) {
	if p.gone {
		return
	}
	if _, err := p.c.KeepLease(ctx, p.id); err != nil {
		p.note(err)
		return
	}
	p.lastSent = time.Now()
}

// note records an answer that means the Run is no longer this desktop's
// to report on.
func (p *reporter) note(err error) {
	switch {
	case errors.Is(err, bakery.ErrSignedOut):
		p.signedOut, p.gone = true, true
	case errors.Is(err, bakery.ErrNotFound), bakery.IsStatus(err, http.StatusConflict), bakery.IsStatus(err, http.StatusForbidden):
		p.gone = true
	}
	if p.gone {
		p.pending = nil
	}
}
