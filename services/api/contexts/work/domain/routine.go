package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jevido/bakery/services/api/app/cron"
)

// MaxRoutineDescription is the longest Markdown description of a Routine,
// in characters.
const MaxRoutineDescription = 20000

// RoutineStatus is whether a Routine's Schedule triggers fire.
type RoutineStatus string

const (
	ActiveRoutine   RoutineStatus = "active"
	PausedRoutine   RoutineStatus = "paused"
	ArchivedRoutine RoutineStatus = "archived"
)

// RoutineStatuses lists every Routine status.
var RoutineStatuses = []RoutineStatus{ActiveRoutine, PausedRoutine, ArchivedRoutine}

// ParseRoutineStatus reads a Routine status by its wire key.
func ParseRoutineStatus(s string) (RoutineStatus, error) {
	if st := RoutineStatus(s); slices.Contains(RoutineStatuses, st) {
		return st, nil
	}
	return "", invalid("status", "status must be active, paused or archived")
}

// ConcurrencyPolicy is what a Routine run does while the Routine has a
// Live execution Issue.
type ConcurrencyPolicy string

const (
	CoalesceIfActive ConcurrencyPolicy = "coalesce_if_active"
	SkipIfActive     ConcurrencyPolicy = "skip_if_active"
	AlwaysEnqueue    ConcurrencyPolicy = "always_enqueue"
)

// ConcurrencyPolicies lists every Concurrency policy, the default first.
var ConcurrencyPolicies = []ConcurrencyPolicy{CoalesceIfActive, SkipIfActive, AlwaysEnqueue}

// ParseConcurrencyPolicy reads a Concurrency policy by its wire key.
func ParseConcurrencyPolicy(s string) (ConcurrencyPolicy, error) {
	if p := ConcurrencyPolicy(s); slices.Contains(ConcurrencyPolicies, p) {
		return p, nil
	}
	return "", invalid("concurrency_policy", "concurrency policy must be coalesce_if_active, skip_if_active or always_enqueue")
}

// CatchUpPolicy is what the scheduler does with the ticks of a Schedule
// trigger it missed.
type CatchUpPolicy string

const (
	SkipMissed           CatchUpPolicy = "skip_missed"
	EnqueueMissedWithCap CatchUpPolicy = "enqueue_missed_with_cap"
)

// CatchUpPolicies lists every Catch-up policy, the default first.
var CatchUpPolicies = []CatchUpPolicy{SkipMissed, EnqueueMissedWithCap}

// ParseCatchUpPolicy reads a Catch-up policy by its wire key.
func ParseCatchUpPolicy(s string) (CatchUpPolicy, error) {
	if p := CatchUpPolicy(s); slices.Contains(CatchUpPolicies, p) {
		return p, nil
	}
	return "", invalid("catch_up_policy", "catch-up policy must be skip_missed or enqueue_missed_with_cap")
}

// ErrRoutineArchived is a change to an archived Routine: nothing leaves
// archived.
var ErrRoutineArchived error = &FieldError{Field: "status", Message: "an archived routine cannot be changed"}

// Routine is recurring work of a Guild: each Routine run makes an
// Execution Issue from its fields. ProjectID, GoalID, ParentIssueID and
// AssigneeAgentID are 0 for none; without an Agent assignee it is a Draft.
// LastTriggeredAt is nil until it first runs.
type Routine struct {
	ID                uint64
	GuildID           uint64
	ProjectID         uint64
	GoalID            uint64
	ParentIssueID     uint64
	AssigneeAgentID   uint64
	Title             string
	Description       string
	Priority          Priority
	Status            RoutineStatus
	ConcurrencyPolicy ConcurrencyPolicy
	CatchUpPolicy     CatchUpPolicy
	CreatedBy         Actor
	LastTriggeredAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewRoutine is an active Routine of the Guild with the default Priority
// and policies, without an Agent assignee, Project, Goal or parent yet.
func NewRoutine(guildID uint64, createdBy Actor, title, description string) (Routine, error) {
	r := Routine{
		GuildID: guildID, CreatedBy: createdBy, Priority: Medium, Status: ActiveRoutine,
		ConcurrencyPolicy: CoalesceIfActive, CatchUpPolicy: SkipMissed,
	}
	if err := r.Rename(title); err != nil {
		return Routine{}, err
	}
	if err := r.Describe(description); err != nil {
		return Routine{}, err
	}
	return r, nil
}

// Draft is true for a Routine without an Agent assignee: it cannot run.
func (r Routine) Draft() bool { return r.AssigneeAgentID == 0 }

// Archived is true once the Routine is archived; it is not changed again.
func (r Routine) Archived() bool { return r.Status == ArchivedRoutine }

func (r *Routine) Rename(title string) error {
	t, err := Title("title", title)
	if err != nil {
		return err
	}
	r.Title = t
	return nil
}

// Describe sets the Markdown description; empty is none.
func (r *Routine) Describe(description string) error {
	if utf8.RuneCountInString(description) > MaxRoutineDescription {
		return invalid("description", "description is at most %d characters", MaxRoutineDescription)
	}
	r.Description = description
	return nil
}

func (r *Routine) SetPriority(p Priority) { r.Priority = p }

// SetStatus moves the Routine between active and paused, or archives it;
// an archived one stays archived.
func (r *Routine) SetStatus(s RoutineStatus) error {
	if r.Archived() && s != ArchivedRoutine {
		return ErrRoutineArchived
	}
	r.Status = s
	return nil
}

func (r *Routine) SetConcurrencyPolicy(p ConcurrencyPolicy) { r.ConcurrencyPolicy = p }

func (r *Routine) SetCatchUpPolicy(p CatchUpPolicy) { r.CatchUpPolicy = p }

// AssignAgent makes the Agent, already known to be one of the Guild's that
// is not terminated, the Agent assignee; 0 makes the Routine a Draft.
func (r *Routine) AssignAgent(agentID uint64) { r.AssigneeAgentID = agentID }

// PlaceIn puts the Routine in a Project, already known to be one of the
// Guild's; 0 is none.
func (r *Routine) PlaceIn(projectID uint64) { r.ProjectID = projectID }

// ServeGoal makes the Goal, which must be the Guild's, the Routine's; nil
// is none.
func (r *Routine) ServeGoal(g *Goal) error {
	if g == nil {
		r.GoalID = 0
		return nil
	}
	if g.GuildID != r.GuildID {
		return invalid("goal_id", "goal not found")
	}
	r.GoalID = g.ID
	return nil
}

// MoveUnder makes the Issue, which must be the Guild's, the parent of the
// Routine's Execution Issues; nil is none.
func (r *Routine) MoveUnder(parent *Issue) error {
	if parent == nil {
		r.ParentIssueID = 0
		return nil
	}
	if parent.GuildID != r.GuildID {
		return invalid("parent_issue_id", "parent issue not found")
	}
	r.ParentIssueID = parent.ID
	return nil
}

// TriggerKind is what makes a Routine trigger fire: its Schedule, or a call
// to Run naming it.
type TriggerKind string

const (
	ScheduleTrigger TriggerKind = "schedule"
	APITrigger      TriggerKind = "api"
)

// TriggerKinds lists every kind of Routine trigger.
var TriggerKinds = []TriggerKind{ScheduleTrigger, APITrigger}

// ParseTriggerKind reads a kind of Routine trigger by its wire key.
func ParseTriggerKind(s string) (TriggerKind, error) {
	if k := TriggerKind(s); slices.Contains(TriggerKinds, k) {
		return k, nil
	}
	return "", invalid("trigger.kind", "kind must be schedule or api")
}

// MaxTriggerLabel is the longest label of a Routine trigger, in
// characters.
const MaxTriggerLabel = 100

// ErrArchivedRoutineTriggers is a Routine trigger added to, or changed on,
// an archived Routine.
var ErrArchivedRoutineTriggers = errors.New("an archived routine's triggers cannot be changed")

// RoutineTrigger is part of its Routine: what makes it run by itself
// (kind schedule) or through the API (kind api). CronExpression and
// Timezone are empty for an api one. NextRunAt is nil while it does not
// fire: off, an api one, or its Routine paused, archived or a Draft.
type RoutineTrigger struct {
	ID             uint64
	GuildID        uint64
	RoutineID      uint64
	Kind           TriggerKind
	Label          string
	Enabled        bool
	CronExpression string
	Timezone       string
	NextRunAt      *time.Time
	LastFiredAt    *time.Time
	LastResult     string
	CreatedBy      Actor
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TriggerSettings changes the fields of a Routine trigger that are not
// nil.
type TriggerSettings struct {
	Label          *string
	CronExpression *string
	Timezone       *string
	Enabled        *bool
}

// NewScheduleTrigger is a schedule Routine trigger of r, its Next run
// counted from now; an empty time zone is UTC.
func NewScheduleTrigger(r Routine, by Actor, label, cronExpression, timezone string, enabled bool, now time.Time) (RoutineTrigger, error) {
	t := RoutineTrigger{GuildID: r.GuildID, RoutineID: r.ID, Kind: ScheduleTrigger, Enabled: enabled, CreatedBy: by, Timezone: "UTC"}
	return t, t.Change(r, TriggerSettings{Label: &label, CronExpression: &cronExpression, Timezone: &timezone}, now)
}

// NewAPITrigger is an api Routine trigger of r.
func NewAPITrigger(r Routine, by Actor, label string, enabled bool) (RoutineTrigger, error) {
	t := RoutineTrigger{GuildID: r.GuildID, RoutineID: r.ID, Kind: APITrigger, Enabled: enabled, CreatedBy: by}
	return t, t.Change(r, TriggerSettings{Label: &label}, time.Time{})
}

// Change sets what s names, each checked, and counts the Next run again
// from now when the cron expression, time zone or enabled changed.
func (t *RoutineTrigger) Change(r Routine, s TriggerSettings, now time.Time) error {
	if r.Archived() {
		return ErrArchivedRoutineTriggers
	}
	next := *t
	if s.Label != nil {
		l := strings.TrimSpace(*s.Label)
		if utf8.RuneCountInString(l) > MaxTriggerLabel {
			return invalid("trigger.label", "label is at most %d characters", MaxTriggerLabel)
		}
		next.Label = l
	}
	if s.Enabled != nil {
		next.Enabled = *s.Enabled
	}
	if next.Kind == APITrigger {
		if s.CronExpression != nil && strings.TrimSpace(*s.CronExpression) != "" {
			return invalid("trigger.cron_expression", "an api trigger has no cron expression")
		}
		if s.Timezone != nil && strings.TrimSpace(*s.Timezone) != "" {
			return invalid("trigger.timezone", "an api trigger has no time zone")
		}
		next.NextRunAt = nil
		*t = next
		return nil
	}
	if s.CronExpression != nil {
		next.CronExpression = strings.TrimSpace(*s.CronExpression)
		if _, err := cron.Parse(next.CronExpression); err != nil {
			return invalid("trigger.cron_expression", "%q is neither a five-field cron expression (minute hour day month weekday) nor every_minute, hourly, daily, weekly, monthly or yearly", next.CronExpression)
		}
	}
	if s.Timezone != nil {
		next.Timezone = strings.TrimSpace(*s.Timezone)
		if next.Timezone == "" {
			next.Timezone = "UTC"
		}
		// Local is the server's own zone, not one a person can name.
		if _, err := cron.Location(next.Timezone); err != nil || next.Timezone == "Local" {
			return invalid("trigger.timezone", "%q is not an IANA time zone, such as Europe/Amsterdam or UTC", next.Timezone)
		}
	}
	scheduleChanged := next.CronExpression != t.CronExpression || next.Timezone != t.Timezone || next.Enabled != t.Enabled
	*t = next
	if scheduleChanged || t.ID == 0 {
		t.Reschedule(r, now)
	}
	return nil
}

// Fires is true while the trigger fires by itself: a schedule one that is
// on, of an active Routine with an Agent assignee.
func (t RoutineTrigger) Fires(r Routine) bool {
	return t.Kind == ScheduleTrigger && t.Enabled && r.Status == ActiveRoutine && !r.Draft()
}

// Reschedule counts the Next run from now, or clears it while the trigger
// does not fire. Ticks missed while it did not fire are never made up.
func (t *RoutineTrigger) Reschedule(r Routine, now time.Time) {
	t.NextRunAt = nil
	if !t.Fires(r) {
		return
	}
	if at, err := cron.Next(t.CronExpression, t.Timezone, now); err == nil {
		t.NextRunAt = &at
	}
}
