package domain

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// RunStatus is a Run status.
type RunStatus string

const (
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
	RunLost      RunStatus = "lost"
	// RunLimited is a Run that stopped because its Hirer's claude login
	// reached its Subscription limit; another Run takes its place.
	RunLimited RunStatus = "limited"
)

// RunStatuses are the glossary's Run statuses, in the order a Run moves.
var RunStatuses = []RunStatus{RunQueued, RunRunning, RunSucceeded, RunFailed, RunCancelled, RunLost, RunLimited}

// ParseRunStatus reads a Run status.
func ParseRunStatus(s string) (RunStatus, error) {
	if slices.Contains(RunStatuses, RunStatus(s)) {
		return RunStatus(s), nil
	}
	return "", invalid("status", "must be one of %s", joined(RunStatuses))
}

// Final reports whether a Run in the status never changes again.
func (s RunStatus) Final() bool {
	return s != RunQueued && s != RunRunning
}

// InvocationSource is what started a Run: Paperclip's
// heartbeat_runs.invocationSource.
type InvocationSource string

const (
	// OnDemand is a Run a person started, with Run or Run heartbeat.
	OnDemand InvocationSource = "on_demand"
	// Timer is a Run the Agent's Heartbeat policy started on its interval.
	Timer InvocationSource = "timer"
	// Assignment is a Run started by an Issue assigned to the Agent.
	Assignment InvocationSource = "assignment"
	// Automation is a Run started by something else in the Guild, such as
	// a comment on the Agent's Issue.
	Automation InvocationSource = "automation"
)

// WakeReason is why a Wake happened, finer than its Invocation source.
type WakeReason string

const (
	Manual           WakeReason = "manual"
	HeartbeatInvoked WakeReason = "heartbeat_invoked"
	IssueAssigned    WakeReason = "issue_assigned"
	IssueCommented   WakeReason = "issue_commented"
	HeartbeatTimer   WakeReason = "heartbeat_timer"
)

// WakeContext is what the Wakes of a Run carry for its prompt: the
// comments that woke it or joined it while it waited.
type WakeContext struct {
	CommentIDs []uint64
}

// WakeRefused refuses a Wake the Agent's Heartbeat policy does not
// allow: anything but its timer while Wake on demand is off.
type WakeRefused struct{}

func (WakeRefused) Error() string {
	return "wake on demand is off"
}

// Lease is how long a running Run lives without a word from its Desktop
// before it is lost.
const Lease = 90 * time.Second

// RunStatusError refuses a change the Run's status does not allow.
type RunStatusError struct {
	Status RunStatus
	Action string
}

func (e *RunStatusError) Error() string {
	return fmt.Sprintf("a %s run cannot be %s", e.Status, e.Action)
}

// Usage is a Run's usage as the claude CLI reports it: tokens, turns,
// how long it took, and its cost shown as an equivalent, never billed.
type Usage struct {
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	Turns             int64
	CostEquivalentUSD float64
	DurationMS        int64
}

// Run is one execution of an Agent on its Hirer's Desktop, optionally on
// one Issue. Ids of 0 are none; ExitCode is nil until claude exits.
type Run struct {
	ID      uint64
	GuildID uint64
	AgentID uint64
	IssueID uint64
	// ProjectID is the Project of the Run's Issue when its Desktop claimed
	// it, kept even if the Issue moves later: Costs and Budgets count it.
	ProjectID        uint64
	InvocationSource InvocationSource
	// WakeReason is the first Wake's, which names the Run; WakeCount
	// counts it and every Wake that joined it while it was queued.
	WakeReason    WakeReason
	WakeCount     int
	WakeContext   WakeContext
	Status        RunStatus
	RequestedByID uint64
	DesktopID     uint64
	RetryOfRunID  uint64
	// KeyHash is the SHA-256 of the Run key Claim gave the Run, "" before;
	// the key itself is never kept.
	KeyHash   string
	Prompt    string
	SessionID string
	ExitCode  *int
	Error     string
	Usage     Usage
	// LimitResetsAt is the Limit reset a limited Run reported, nil for any
	// other.
	LimitResetsAt *time.Time
	// NextSeq is the seq the next Run event must have at least.
	NextSeq        int64
	LeaseExpiresAt *time.Time
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	UpdatedAt      time.Time
}

// Runnable reports whether a Run of the Agent may be started: only an
// idle, running or error one (a running one's Run waits behind it).
func (a Agent) Runnable() error {
	if a.Status != Idle && a.Status != Running && a.Status != Error {
		return &StatusError{Status: a.Status, Action: "run"}
	}
	return nil
}

// Wakeable reports whether a Wake from the source may start a Run of the
// Agent: it must be Runnable, and only its timer wakes it while Wake on
// demand is off.
func (a Agent) Wakeable(source InvocationSource) error {
	if err := a.Runnable(); err != nil {
		return err
	}
	if source != Timer && !a.Heartbeat.WakeOnDemand {
		return WakeRefused{}
	}
	return nil
}

// StartRun queues a Run of the Agent on the Issue (0 for none) for a
// Wake, asked by a Member (0 for none). Its prompt is written when it is
// claimed, so Wakes that join it meanwhile are in it.
func StartRun(a Agent, issueID, requestedByID uint64, source InvocationSource, reason WakeReason, wc WakeContext, at time.Time) (Run, error) {
	if err := a.Wakeable(source); err != nil {
		return Run{}, err
	}
	return Run{
		GuildID: a.GuildID, AgentID: a.ID, IssueID: issueID, InvocationSource: source, WakeReason: reason, WakeCount: 1,
		WakeContext: wc, Status: RunQueued, RequestedByID: requestedByID, NextSeq: 1, CreatedAt: at, UpdatedAt: at,
	}, nil
}

// Join adds a Wake to a queued Run instead of queueing another: the count
// and the comments grow, and the Run keeps its own Invocation source and
// Wake reason.
func (r *Run) Join(wc WakeContext, at time.Time) error {
	if r.Status != RunQueued {
		return &RunStatusError{Status: r.Status, Action: "joined"}
	}
	r.WakeCount++
	for _, id := range wc.CommentIDs {
		if !slices.Contains(r.WakeContext.CommentIDs, id) {
			r.WakeContext.CommentIDs = append(r.WakeContext.CommentIDs, id)
		}
	}
	r.UpdatedAt = at
	return nil
}

// RunKeyPrefix starts every Run key, so a leaked one is recognisable and
// never mistaken for a Desktop key or an API token.
const RunKeyPrefix = "bky_run_"

// ValidRunKey reports whether s has the shape ClaimRun mints: the prefix
// and 24 random bytes in hex.
func ValidRunKey(s string) bool {
	rest, ok := strings.CutPrefix(s, RunKeyPrefix)
	if !ok || len(rest) != 48 {
		return false
	}
	for _, c := range rest {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Claim makes a queued Run running on the Desktop, its Lease starting,
// with the hash of the Run key its Desktop gets and the Project its Issue
// is in now (0 for none), which it keeps from then on.
func (r *Run) Claim(desktopID, projectID uint64, keyHash string, at time.Time) error {
	if r.Status != RunQueued {
		return &RunStatusError{Status: r.Status, Action: "claimed"}
	}
	lease := at.Add(Lease)
	r.Status, r.DesktopID, r.ProjectID, r.KeyHash, r.StartedAt, r.LeaseExpiresAt, r.UpdatedAt = RunRunning, desktopID, projectID, keyHash, &at, &lease, at
	return nil
}

// KeepLease renews a running Run's Lease.
func (r *Run) KeepLease(at time.Time) error {
	if r.Status != RunRunning {
		return &RunStatusError{Status: r.Status, Action: "kept"}
	}
	lease := at.Add(Lease)
	r.LeaseExpiresAt, r.UpdatedAt = &lease, at
	return nil
}

// Cancel ends a queued or running Run as cancelled.
func (r *Run) Cancel(at time.Time) error {
	if r.Status.Final() {
		return &RunStatusError{Status: r.Status, Action: "cancelled"}
	}
	r.end(RunCancelled, at)
	return nil
}

// CancelBecause cancels a queued or running Run with the reason as its
// error, as a Budget's Hard stop does.
func (r *Run) CancelBecause(reason string, at time.Time) error {
	if err := r.Cancel(at); err != nil {
		return err
	}
	r.Error = reason
	return nil
}

func (r *Run) end(s RunStatus, at time.Time) {
	r.Status, r.FinishedAt, r.LeaseExpiresAt, r.UpdatedAt = s, &at, nil, at
}

// Finish ends a running Run as succeeded, failed or cancelled, with its
// Run usage, set only here, and an error message when it failed.
func (r *Run) Finish(s RunStatus, u Usage, exitCode *int, message string, at time.Time) error {
	if s != RunSucceeded && s != RunFailed && s != RunCancelled {
		return invalid("status", "must be one of succeeded, failed, cancelled")
	}
	if r.Status != RunRunning {
		return &RunStatusError{Status: r.Status, Action: "finished"}
	}
	r.end(s, at)
	r.Usage, r.ExitCode, r.Error = u, exitCode, strings.TrimSpace(message)
	return nil
}

// Lose ends a running Run whose Lease ran out as lost, and answers the
// queued Run that takes its place.
func (r *Run) Lose(at time.Time) (Run, error) {
	if r.Status != RunRunning {
		return Run{}, &RunStatusError{Status: r.Status, Action: "lost"}
	}
	r.end(RunLost, at)
	r.Error = "the desktop stopped reporting"
	return r.replacement(at), nil
}

// MaxLimitWait is how far ahead a Limit reset may be: claude's longest
// usage window is a week, and a day more allows for clocks.
const MaxLimitWait = 8 * 24 * time.Hour

// Limit ends a running Run whose Hirer's claude login reached its
// Subscription limit as limited, with its Run usage and the Limit reset,
// and answers the queued Run that takes its place, as Lose does.
func (r *Run) Limit(u Usage, exitCode *int, message string, resetsAt, at time.Time) (Run, error) {
	if !resetsAt.After(at) {
		return Run{}, invalid("limit_resets_at", "must be in the future")
	}
	if resetsAt.Sub(at) > MaxLimitWait {
		return Run{}, invalid("limit_resets_at", "must be at most 8 days ahead")
	}
	if r.Status != RunRunning {
		return Run{}, &RunStatusError{Status: r.Status, Action: "finished"}
	}
	r.end(RunLimited, at)
	r.Usage, r.ExitCode, r.Error = u, exitCode, strings.TrimSpace(message)
	if r.Error == "" {
		r.Error = "the subscription limit was reached"
	}
	resets := resetsAt.UTC()
	r.LimitResetsAt = &resets
	return r.replacement(at), nil
}

// replacement is the queued Run that takes a lost or limited Run's place:
// the same Agent, Issue, Invocation source, Wake reason and wake context.
func (r *Run) replacement(at time.Time) Run {
	return Run{
		GuildID: r.GuildID, AgentID: r.AgentID, IssueID: r.IssueID, InvocationSource: r.InvocationSource,
		WakeReason: r.WakeReason, WakeCount: 1, WakeContext: WakeContext{CommentIDs: slices.Clone(r.WakeContext.CommentIDs)},
		Status: RunQueued, RequestedByID: r.RequestedByID, RetryOfRunID: r.ID, NextSeq: 1, CreatedAt: at, UpdatedAt: at,
	}
}

// DesktopLimit is one Desktop's claude login at its Subscription limit
// until ResetsAt, as its Desktop reported it.
type DesktopLimit struct {
	DesktopID  uint64
	MemberID   uint64
	ResetsAt   time.Time
	ReportedAt time.Time
}

// Active reports whether the limit still holds at the time.
func (l DesktopLimit) Active(at time.Time) bool {
	return l.DesktopID != 0 && at.Before(l.ResetsAt)
}

// EventKind is what a Run event tells.
type EventKind string

// EventKinds are the glossary's Run event kinds.
var EventKinds = []EventKind{"init", "assistant", "thinking", "tool_call", "tool_result", "result", "stderr", "system"}

// RunEvent is one thing a Run's claude printed, in order by Seq. Payload
// is its JSON as the Desktop sent it.
type RunEvent struct {
	RunID     uint64
	Seq       int64
	Kind      EventKind
	Payload   []byte
	CreatedAt time.Time
}

// Append keeps the Run events of a running Run that it does not have yet:
// one with a seq below NextSeq is one it has (a no-op), the rest must
// follow on without a gap, from NextSeq, and have a known kind. It answers
// those to store.
func (r *Run) Append(events []RunEvent, at time.Time) ([]RunEvent, error) {
	if r.Status != RunRunning {
		return nil, &RunStatusError{Status: r.Status, Action: "reported on"}
	}
	var out []RunEvent
	next := r.NextSeq
	for _, e := range events {
		if e.Seq < r.NextSeq {
			continue
		}
		if e.Seq != next {
			return nil, &SeqError{Expected: next, Got: e.Seq}
		}
		if !slices.Contains(EventKinds, e.Kind) {
			return nil, invalid("events", "kind must be one of %s", joined(EventKinds))
		}
		e.RunID, e.CreatedAt = r.ID, at
		out = append(out, e)
		next = e.Seq + 1
	}
	r.NextSeq, r.UpdatedAt = next, at
	return out, nil
}

// SeqError refuses Run events that do not follow on from the ones a Run
// has: the Desktop sends again from Expected.
type SeqError struct {
	Expected, Got int64
}

func (e *SeqError) Error() string {
	return fmt.Sprintf("events: expected seq %d, got %d", e.Expected, e.Got)
}

// SessionOf is the claude session id an init Run event's payload names,
// "" when it names none.
func SessionOf(e RunEvent) string {
	if e.Kind != "init" {
		return ""
	}
	var p struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(e.Payload, &p) != nil {
		return ""
	}
	return strings.TrimSpace(p.SessionID)
}

// MaxChainRetries is how many times a lost Run is queued again, counting
// the lost Runs along its retry_of_run_id chain, before the last one stays
// lost. Limited Runs do not count: they always wait for the Limit reset.
const MaxChainRetries = 3

// RunStarted is published when a Wake queued a new Run, other than the
// timer's.
type RunStarted struct {
	Run     Run
	Agent   Agent
	ActorID uint64
	// Identifier is the Issue identifier of the Run's Issue, if any.
	Identifier string
}

// RunFinished is published when a Run reached a final Run status.
type RunFinished struct {
	Run     Run
	Agent   Agent
	ActorID uint64
}
