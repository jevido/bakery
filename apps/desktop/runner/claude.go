package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	cmd := exec.Command(bin, claudeArgs...)
	cmd.Dir = dir
	cmd.Env = claudeEnv(os.Environ())
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
