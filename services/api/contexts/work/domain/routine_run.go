package domain

import (
	"errors"
	"slices"
	"time"
)

// RoutineRunSource is what made a Routine run: its Schedule, a person
// pressing Run, a call to Run naming an api Routine trigger, or a Webhook
// delivery to a webhook one.
type RoutineRunSource string

const (
	ScheduleSource RoutineRunSource = "schedule"
	ManualSource   RoutineRunSource = "manual"
	APISource      RoutineRunSource = "api"
	WebhookSource  RoutineRunSource = "webhook"
)

// RoutineRunSources lists every source of a Routine run.
var RoutineRunSources = []RoutineRunSource{ScheduleSource, ManualSource, APISource, WebhookSource}

// RoutineRunStatus is a Routine run status.
type RoutineRunStatus string

const (
	RunReceived     RoutineRunStatus = "received"
	RunIssueCreated RoutineRunStatus = "issue_created"
	RunCoalesced    RoutineRunStatus = "coalesced"
	RunSkipped      RoutineRunStatus = "skipped"
	RunCompleted    RoutineRunStatus = "completed"
	RunFailed       RoutineRunStatus = "failed"
)

// RoutineRunStatuses lists every Routine run status, in the order a
// Routine run moves.
var RoutineRunStatuses = []RoutineRunStatus{RunReceived, RunIssueCreated, RunCoalesced, RunSkipped, RunCompleted, RunFailed}

var (
	// ErrRoutineArchivedRun is Run on an archived Routine.
	ErrRoutineArchivedRun = errors.New("an archived routine does not run")
	// ErrRoutinePaused is a Schedule or Webhook trigger firing for a
	// paused Routine.
	ErrRoutinePaused = errors.New("a paused routine does not run by itself")
	// ErrTriggerDisabled is a Routine run naming a Routine trigger that is
	// off.
	ErrTriggerDisabled = errors.New("the routine trigger is disabled")
	// ErrNotRoutinesTrigger is a Routine run naming another Routine's
	// trigger.
	ErrNotRoutinesTrigger = errors.New("the trigger is not this routine's")
)

// ReceiveRoutineRun is a Routine run of r from source, named by trigger
// (nil for none), triggered by the Member or Agent (nobody for a
// Schedule or a Webhook delivery) at at. It refuses an archived Routine, a
// Draft, a paused one for a Schedule or a Webhook delivery, and a trigger
// that is not r's, is off, or is not of the source's kind.
func ReceiveRoutineRun(r Routine, trigger *RoutineTrigger, source RoutineRunSource, by Actor, at time.Time) (RoutineRun, error) {
	if !slices.Contains(RoutineRunSources, source) {
		return RoutineRun{}, invalid("source", "source must be schedule, manual, api or webhook")
	}
	if r.Archived() {
		return RoutineRun{}, ErrRoutineArchivedRun
	}
	if r.Draft() {
		return RoutineRun{}, invalid("assignee_agent_id", "default agent required")
	}
	if (source == ScheduleSource || source == WebhookSource) && r.Status == PausedRoutine {
		return RoutineRun{}, ErrRoutinePaused
	}
	rr := RoutineRun{GuildID: r.GuildID, RoutineID: r.ID, Source: source, Status: RunReceived, TriggeredAt: at, TriggeredBy: by}
	if trigger == nil {
		if source != ManualSource {
			return RoutineRun{}, invalid("trigger_id", "a %s run names its %s trigger", source, source)
		}
		return rr, nil
	}
	if trigger.RoutineID != r.ID {
		return RoutineRun{}, ErrNotRoutinesTrigger
	}
	if !trigger.Enabled {
		return RoutineRun{}, ErrTriggerDisabled
	}
	if want := map[RoutineRunSource]TriggerKind{ScheduleSource: ScheduleTrigger, APISource: APITrigger, WebhookSource: WebhookTrigger}[source]; trigger.Kind != want {
		return RoutineRun{}, invalid("trigger_id", "a %s run needs a trigger of kind %s", source, source)
	}
	rr.TriggerID = trigger.ID
	return rr, nil
}

// RoutineRun is one firing of a Routine. TriggerID, LinkedIssueID and
// CoalescedIntoRunID are 0 for none; TriggeredBy is nobody for a
// Schedule or a Webhook delivery; IdempotencyKey is the Webhook
// delivery's, "" for none; Variables are the values its Routine
// variables and the built-in ones took, nil when its Routine has no
// placeholders; CompletedAt is set once it no longer follows
// its Execution Issue.
type RoutineRun struct {
	ID                 uint64
	GuildID            uint64
	RoutineID          uint64
	TriggerID          uint64
	Source             RoutineRunSource
	Status             RoutineRunStatus
	TriggeredAt        time.Time
	LinkedIssueID      uint64
	CoalescedIntoRunID uint64
	FailureReason      string
	TriggeredBy        Actor
	IdempotencyKey     string
	Variables          map[string]any
	CompletedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// LinkToLive ends the Routine run on its Routine's Live execution Issue
// instead of a new one: coalesced, or skipped under skip_if_active.
// Under always_enqueue it changes nothing and answers false.
func (rr *RoutineRun) LinkToLive(p ConcurrencyPolicy, live Issue, at time.Time) bool {
	switch p {
	case CoalesceIfActive:
		rr.Status = RunCoalesced
	case SkipIfActive:
		rr.Status = RunSkipped
	default:
		return false
	}
	rr.LinkedIssueID, rr.CoalescedIntoRunID, rr.CompletedAt = live.ID, live.OriginRoutineRunID, &at
	return true
}

// IssueCreated links the Routine run to the Execution Issue it created.
func (rr *RoutineRun) IssueCreated(issueID uint64) {
	rr.Status, rr.LinkedIssueID = RunIssueCreated, issueID
}

// Fail ends the Routine run failed, for the reason.
func (rr *RoutineRun) Fail(reason string, at time.Time) {
	rr.Status, rr.FailureReason, rr.CompletedAt = RunFailed, reason, &at
}

// Follow moves the Routine run with its Execution Issue's status: done
// completes it, cancelled or blocked fails it, and reopening one of those
// makes it issue_created again. A coalesced or skipped one does not
// follow; changed is false when nothing moved.
func (rr *RoutineRun) Follow(st IssueStatus, at time.Time) (changed bool) {
	if rr.Status != RunIssueCreated && rr.Status != RunCompleted && rr.Status != RunFailed || rr.LinkedIssueID == 0 {
		return false
	}
	before := *rr
	switch st {
	case Done:
		rr.Status, rr.FailureReason, rr.CompletedAt = RunCompleted, "", &at
	case IssueCancelled:
		rr.Fail("Execution issue cancelled", at)
	case Blocked:
		rr.Fail("Execution issue blocked", at)
	default:
		rr.Status, rr.FailureReason, rr.CompletedAt = RunIssueCreated, "", nil
	}
	return rr.Status != before.Status || rr.FailureReason != before.FailureReason
}

// IssueDeleted unlinks the Routine run from its deleted Execution Issue,
// failing it unless it was already done or failed.
func (rr *RoutineRun) IssueDeleted(at time.Time) {
	rr.LinkedIssueID = 0
	if rr.Status == RunIssueCreated {
		rr.Fail("Execution issue deleted", at)
	}
}
