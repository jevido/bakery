// Package runner is the Desktop app's Runner: for every connected Bakery
// it follows the person's queued Runs, claims them, runs `claude` for each
// on this computer, reports what it prints as Run events, keeps the Lease
// and finishes the Run. The server never runs an Agent; this is where they
// run. See README.md, "Runs".
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jevido/bakery/apps/desktop/bakery"
	"github.com/jevido/bakery/apps/desktop/store"
)

// How often the Runner reads bakeries.json again, so a Bakery connected or
// signed out after it started is noticed.
var storeEvery = 5 * time.Second

// defaultMax is how many Runs one desktop runs at once when
// BAKERY_RUNNER_MAX does not say.
const defaultMax = 2

// Runner runs the person's Runs from every connected Bakery.
type Runner struct {
	// Store holds the connected Bakeries and their Desktop keys.
	Store *store.Store
	// Claude is the claude binary; empty means BAKERY_CLAUDE, else
	// `claude` on PATH.
	Claude string
	// Home is where each Run gets its working directory, runs/<run id>.
	Home string
	// Max is how many Runs run at once; 0 means BAKERY_RUNNER_MAX, else 2.
	Max int
	// Events, when set, is told about each Run on this desktop: `runs`
	// with a RunUpdate when it starts, reports and ends.
	Events func(name string, data any)
	// Logf logs one line per claim and finish; nil means the log package.
	Logf func(format string, args ...any)

	mu      sync.Mutex
	running map[runKey]*execution
	watched map[string]watch
	wg      sync.WaitGroup
}

// RunUpdate is the `runs` event: a Run on this desktop and where it
// stands (running, succeeded, failed, cancelled, or stopped when the
// Bakery no longer has it running).
type RunUpdate struct {
	Address string `json:"address"`
	GuildID uint64 `json:"guild_id"`
	AgentID uint64 `json:"agent_id"`
	RunID   uint64 `json:"run_id"`
	Status  string `json:"status"`
	Events  int64  `json:"events"`
}

// LocalEvent is one Run event this desktop's claude generated, with when it
// generated it; the Transcript the frontend draws needs no more.
type LocalEvent struct {
	Seq       int64           `json:"seq"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// LocalRun is a Run this desktop executes right now: LocalRuns reports it so
// the frontend draws its Transcript without a round trip to the Bakery.
type LocalRun struct {
	Address   string           `json:"address"`
	Guild     bakery.Named     `json:"guild"`
	Agent     bakery.RunAgent  `json:"agent"`
	RunID     uint64           `json:"run_id"`
	Issue     *bakery.RunIssue `json:"issue"`
	Status    string           `json:"status"`
	StartedAt time.Time        `json:"started_at"`
	Events    []LocalEvent     `json:"events"`
}

type runKey struct {
	address string
	id      uint64
}

// execution is a Run this desktop holds, and what it has reported so far
// for LocalRuns to show without a round trip to the Bakery.
type execution struct {
	agentID   uint64
	cancelled chan struct{}
	once      sync.Once

	mu    sync.Mutex
	local LocalRun
}

func (e *execution) cancel() { e.once.Do(func() { close(e.cancelled) }) }

// start records the Run this execution holds, once claimed.
func (e *execution) start(address string, run bakery.DesktopRun) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.local = LocalRun{Address: address, Guild: run.Guild, Agent: run.Agent, RunID: run.ID, Issue: run.Issue, Status: "running", StartedAt: time.Now()}
}

// record appends events this execution's claude printed, in seq order.
func (e *execution) record(events []LocalEvent) {
	if len(events) == 0 {
		return
	}
	e.mu.Lock()
	e.local.Events = append(e.local.Events, events...)
	e.mu.Unlock()
}

// snapshot is this execution's LocalRun right now, or the zero value before
// start was called (claimed but not yet recorded: offer filters it out).
func (e *execution) snapshot() LocalRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	l := e.local
	l.Events = append([]LocalEvent(nil), l.Events...)
	return l
}

// watch is one Bakery's stream being followed, with the key it uses.
type watch struct {
	key    string
	cancel context.CancelFunc
}

// Start follows every connected Bakery until ctx ends. When ctx ends,
// the claude processes still running are stopped and their Runs left
// alone: the Bakery marks them lost and queues them again.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	r.running, r.watched = map[runKey]*execution{}, map[string]watch{}
	r.mu.Unlock()
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			r.follow(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(storeEvery):
			}
		}
	}()
}

// Wait returns once Start's ctx ended and every Run it held stopped.
func (r *Runner) Wait() { r.wg.Wait() }

// follow starts watching the connected Bakeries not yet watched and stops
// watching the ones removed, signed out or holding another key.
func (r *Runner) follow(ctx context.Context) {
	f, err := r.Store.Load()
	if err != nil {
		r.logf("runner: reading the connected Bakeries: %v", err)
		return
	}
	want := map[string]store.Bakery{}
	for _, b := range f.Bakeries {
		if !b.SignedOut && b.Key != "" {
			want[b.Address] = b
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for address, w := range r.watched {
		if b, ok := want[address]; !ok || b.Key != w.key {
			w.cancel()
			delete(r.watched, address)
		}
	}
	if ctx.Err() != nil {
		return
	}
	for address, b := range want {
		if _, ok := r.watched[address]; ok {
			continue
		}
		wctx, cancel := context.WithCancel(ctx)
		r.watched[address] = watch{key: b.Key, cancel: cancel}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.watch(wctx, bakery.New(b.Address, b.Key))
		}()
	}
}

// watch offers the Bakery's queued Runs to this desktop, first from its
// list and then from its stream, until ctx ends or the key stops working.
func (r *Runner) watch(ctx context.Context, c *bakery.Client) {
	runs, err := c.DesktopRuns(ctx)
	if err == nil {
		r.offer(ctx, c, runs)
	} else if !errors.Is(err, bakery.ErrSignedOut) && ctx.Err() == nil {
		r.logf("runner: %s: %v", c.Address, err)
	}
	if err == nil || !errors.Is(err, bakery.ErrSignedOut) {
		err = c.WatchRuns(ctx, bakery.RunsWatcher{
			Runs:   func(runs []bakery.DesktopRun) { r.offer(ctx, c, runs) },
			Cancel: func(id uint64) { r.cancel(c.Address, id) },
		})
	}
	if errors.Is(err, bakery.ErrSignedOut) {
		r.logf("runner: %s signed this desktop out", c.Address)
		if err := r.Store.MarkSignedOut(c.Address); err != nil {
			r.logf("runner: %v", err)
		}
	}
}

// offer claims each queued Run this desktop has room for: none of its
// Agent's Runs already running here, and fewer than Max running at all.
// A Run another Desktop of the person claimed first is skipped quietly.
func (r *Runner) offer(ctx context.Context, c *bakery.Client, runs []bakery.DesktopRun) {
	for _, run := range runs {
		if run.Status != "queued" || ctx.Err() != nil {
			continue
		}
		key := runKey{c.Address, run.ID}
		e, ok := r.reserve(key, run.Agent.ID)
		if !ok {
			continue
		}
		claimed, err := c.ClaimRun(ctx, run.ID)
		if err != nil {
			r.release(key)
			if !errors.Is(err, bakery.ErrNotFound) && !bakery.IsStatus(err, http.StatusConflict) &&
				!bakery.IsStatus(err, http.StatusForbidden) && ctx.Err() == nil {
				r.logf("runner: claiming run %d on %s: %v", run.ID, c.Address, err)
			}
			continue
		}
		e.start(c.Address, claimed)
		r.logf("runner: claimed run %d of %s (%s) on %s", claimed.ID, claimed.Agent.Name, claimed.Guild.Name, c.Address)
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			defer r.release(key)
			r.execute(ctx, c, claimed, e)
		}()
	}
}

func (r *Runner) reserve(key runKey, agentID uint64) (*execution, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.running[key]; ok || len(r.running) >= r.max() {
		return nil, false
	}
	for k, e := range r.running {
		if k.address == key.address && e.agentID == agentID {
			return nil, false
		}
	}
	e := &execution{agentID: agentID, cancelled: make(chan struct{})}
	r.running[key] = e
	return e, true
}

func (r *Runner) release(key runKey) {
	r.mu.Lock()
	delete(r.running, key)
	r.mu.Unlock()
}

// cancel stops the Run id of the Bakery at address, when this desktop
// runs it.
func (r *Runner) cancel(address string, id uint64) {
	r.mu.Lock()
	e := r.running[runKey{address, id}]
	r.mu.Unlock()
	if e != nil {
		e.cancel()
	}
}

func (r *Runner) max() int {
	if r.Max > 0 {
		return r.Max
	}
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BAKERY_RUNNER_MAX"))); err == nil && n > 0 {
		return n
	}
	return defaultMax
}

// LocalRuns is the Runs this Runner executes right now, across every
// connected Bakery, oldest first: for the frontend's Agent page and its
// "Runs on this desktop" without a round trip to a Bakery. A Run claimed
// but not yet started (no claude output yet) is still included, with no
// events.
func (r *Runner) LocalRuns() []LocalRun {
	r.mu.Lock()
	es := make([]*execution, 0, len(r.running))
	for _, e := range r.running {
		es = append(es, e)
	}
	r.mu.Unlock()
	out := make([]LocalRun, 0, len(es))
	for _, e := range es {
		if l := e.snapshot(); l.RunID != 0 {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (r *Runner) emit(c *bakery.Client, run bakery.DesktopRun, status string, events int64) {
	if r.Events != nil {
		r.Events("runs", RunUpdate{Address: c.Address, GuildID: run.Guild.ID, AgentID: run.Agent.ID, RunID: run.ID, Status: status, Events: events})
	}
}

func (r *Runner) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}
