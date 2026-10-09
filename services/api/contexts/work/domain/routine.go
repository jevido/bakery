package domain

import (
	"slices"
	"time"
	"unicode/utf8"
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
