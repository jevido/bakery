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
)

// RunStatuses are the glossary's Run statuses, in the order a Run moves.
var RunStatuses = []RunStatus{RunQueued, RunRunning, RunSucceeded, RunFailed, RunCancelled, RunLost}

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

// InvocationSource is why a Run was started.
type InvocationSource string

// OnDemand is a Run a person started with Run.
const OnDemand InvocationSource = "on_demand"

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
	ID               uint64
	GuildID          uint64
	AgentID          uint64
	IssueID          uint64
	InvocationSource InvocationSource
	Status           RunStatus
	RequestedByID    uint64
	DesktopID        uint64
	RetryOfRunID     uint64
	Prompt           string
	SessionID        string
	ExitCode         *int
	Error            string
	Usage            Usage
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

// StartRun queues a Run of the Agent on the Issue (0 for none), asked by
// a Member.
func StartRun(a Agent, issueID, requestedByID uint64, source InvocationSource, prompt string, at time.Time) (Run, error) {
	if err := a.Runnable(); err != nil {
		return Run{}, err
	}
	return Run{
		GuildID: a.GuildID, AgentID: a.ID, IssueID: issueID, InvocationSource: source, Status: RunQueued,
		RequestedByID: requestedByID, Prompt: prompt, NextSeq: 1, CreatedAt: at, UpdatedAt: at,
	}, nil
}

// Claim makes a queued Run running on the Desktop, its Lease starting.
func (r *Run) Claim(desktopID uint64, at time.Time) error {
	if r.Status != RunQueued {
		return &RunStatusError{Status: r.Status, Action: "claimed"}
	}
	lease := at.Add(Lease)
	r.Status, r.DesktopID, r.StartedAt, r.LeaseExpiresAt, r.UpdatedAt = RunRunning, desktopID, &at, &lease, at
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
	return Run{
		GuildID: r.GuildID, AgentID: r.AgentID, IssueID: r.IssueID, InvocationSource: r.InvocationSource, Status: RunQueued,
		RequestedByID: r.RequestedByID, RetryOfRunID: r.ID, Prompt: r.Prompt, NextSeq: 1, CreatedAt: at, UpdatedAt: at,
	}, nil
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
// along its retry_of_run_id chain, before the last one stays lost.
const MaxChainRetries = 3

// RunStarted is published when a Member has started a Run.
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
