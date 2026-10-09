package domain

import (
	"errors"
	"time"
)

// ErrStaleRoutineRevision is a change to a Routine made on a Base revision
// that is no longer its newest Routine revision: someone else saved first.
var ErrStaleRoutineRevision = errors.New("routine was updated by someone else")

// RoutineRevision is one kept version of a Routine and its Routine
// triggers, numbered 1, 2, … per Routine without gaps. It is never
// changed. Author is nobody for revision 1 of a Routine that existed
// before revisions; RestoredFromID is 0 unless it was a Restore.
type RoutineRevision struct {
	ID             uint64
	GuildID        uint64
	RoutineID      uint64
	Number         int
	Title          string
	Description    string
	Snapshot       RoutineSnapshot
	ChangeSummary  string
	RestoredFromID uint64
	Author         Actor
	CreatedAt      time.Time
}

// RoutineSnapshot is what a Routine revision keeps of the Routine and its
// triggers. Version is its shape, 1 for now, as Paperclip's. The JSON is
// the one kept and the one answered.
type RoutineSnapshot struct {
	Version  int               `json:"version"`
	Routine  SnapshotRoutine   `json:"routine"`
	Triggers []SnapshotTrigger `json:"triggers"`
}

// SnapshotRoutine is a Routine's fields in its Snapshot; the ids are 0
// for none.
type SnapshotRoutine struct {
	ID                uint64             `json:"id"`
	GuildID           uint64             `json:"guild_id"`
	ProjectID         uint64             `json:"project_id"`
	GoalID            uint64             `json:"goal_id"`
	ParentIssueID     uint64             `json:"parent_issue_id"`
	AssigneeAgentID   uint64             `json:"assignee_agent_id"`
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	Priority          Priority           `json:"priority"`
	Status            RoutineStatus      `json:"status"`
	ConcurrencyPolicy ConcurrencyPolicy  `json:"concurrency_policy"`
	CatchUpPolicy     CatchUpPolicy      `json:"catch_up_policy"`
	Variables         []SnapshotVariable `json:"variables"`
}

// SnapshotVariable is a Routine variable's definition in a Snapshot.
type SnapshotVariable struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Default  any      `json:"default_value"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
}

// SnapshotTrigger is a Routine trigger in a Snapshot: never its secret,
// Next run or last Webhook delivery. CronExpression and Timezone are
// empty, and PublicID, SigningMode and ReplayWindowSec zero, where the
// kind has none.
type SnapshotTrigger struct {
	ID              uint64      `json:"id"`
	Kind            TriggerKind `json:"kind"`
	Label           string      `json:"label"`
	Enabled         bool        `json:"enabled"`
	CronExpression  string      `json:"cron_expression"`
	Timezone        string      `json:"timezone"`
	PublicID        string      `json:"public_id"`
	SigningMode     SigningMode `json:"signing_mode"`
	ReplayWindowSec int         `json:"replay_window_sec"`
}

// SnapshotOf is the Snapshot of r and its triggers, ts in the order they
// were added.
func SnapshotOf(r Routine, ts []RoutineTrigger) RoutineSnapshot {
	vars := make([]SnapshotVariable, len(r.Variables))
	for n, v := range r.Variables {
		opts := v.Options
		if opts == nil {
			opts = []string{}
		}
		vars[n] = SnapshotVariable{Name: v.Name, Label: v.Label, Type: string(v.Type), Default: v.Default, Required: v.Required, Options: opts}
	}
	triggers := make([]SnapshotTrigger, len(ts))
	for n, t := range ts {
		triggers[n] = SnapshotTrigger{
			ID: t.ID, Kind: t.Kind, Label: t.Label, Enabled: t.Enabled, CronExpression: t.CronExpression, Timezone: t.Timezone,
			PublicID: t.PublicID, SigningMode: t.SigningMode, ReplayWindowSec: t.ReplayWindowSec,
		}
	}
	return RoutineSnapshot{
		Version: 1,
		Routine: SnapshotRoutine{
			ID: r.ID, GuildID: r.GuildID, ProjectID: r.ProjectID, GoalID: r.GoalID, ParentIssueID: r.ParentIssueID,
			AssigneeAgentID: r.AssigneeAgentID, Title: r.Title, Description: r.Description, Priority: r.Priority, Status: r.Status,
			ConcurrencyPolicy: r.ConcurrencyPolicy, CatchUpPolicy: r.CatchUpPolicy, Variables: vars,
		},
		Triggers: triggers,
	}
}

// NewRoutineRevision is the revision after the Routine's newest one, of r
// and its triggers as they are now.
func NewRoutineRevision(r Routine, ts []RoutineTrigger, by Actor, changeSummary string, restoredFromID uint64, at time.Time) RoutineRevision {
	return RoutineRevision{
		GuildID: r.GuildID, RoutineID: r.ID, Number: r.LatestRevisionNumber + 1, Title: r.Title, Description: r.Description,
		Snapshot: SnapshotOf(r, ts), ChangeSummary: changeSummary, RestoredFromID: restoredFromID, Author: by, CreatedAt: at,
	}
}

// CheckBaseRevision refuses a change made on a Base revision that is not
// the Routine's newest Routine revision.
func (r Routine) CheckBaseRevision(base uint64) error {
	if base != r.LatestRevisionID {
		return ErrStaleRoutineRevision
	}
	return nil
}

// ErrRestoreArchivedRoutine is a Restore of a revision of an archived
// Routine, which is not changed again.
var ErrRestoreArchivedRoutine = errors.New("an archived routine cannot be restored")

// Restore puts back the title, description, Priority, Routine status,
// policies and Routine variables of the Snapshot, each through its setter
// so every invariant still holds. The Project, Goal, parent Issue and
// Agent assignee are the caller's to check and set.
func (r *Routine) Restore(s SnapshotRoutine) error {
	if r.Archived() {
		return ErrRestoreArchivedRoutine
	}
	if s.Status != ActiveRoutine && s.Status != PausedRoutine {
		return ErrRestoreArchivedRoutine
	}
	if err := r.Rename(s.Title); err != nil {
		return err
	}
	if err := r.Describe(s.Description); err != nil {
		return err
	}
	vars := make([]RoutineVariable, len(s.Variables))
	for n, v := range s.Variables {
		vars[n] = RoutineVariable{Name: v.Name, Label: v.Label, Type: VariableType(v.Type), Default: v.Default, Required: v.Required, Options: v.Options}
	}
	if err := r.SetVariables(vars); err != nil {
		return err
	}
	r.SetPriority(s.Priority)
	r.SetConcurrencyPolicy(s.ConcurrencyPolicy)
	r.SetCatchUpPolicy(s.CatchUpPolicy)
	return r.SetStatus(s.Status)
}

// RestoreTrigger puts the Routine trigger t of r back as the Snapshot s
// has it, keeping its Public id and secret, and counts its Next run again
// from now. A trigger that is gone since is a zero t of the Snapshot's
// id; a webhook one is given the new Public id and secret.
func RestoreTrigger(r Routine, t RoutineTrigger, s SnapshotTrigger, by Actor, publicID, secret string, now time.Time) (RoutineTrigger, error) {
	if t.ID == 0 {
		t = RoutineTrigger{ID: s.ID, GuildID: r.GuildID, RoutineID: r.ID, Kind: s.Kind, CreatedBy: by}
		if s.Kind == WebhookTrigger {
			t.PublicID, t.Secret = publicID, secret
		}
	}
	set := TriggerSettings{Label: &s.Label, Enabled: &s.Enabled}
	switch s.Kind {
	case ScheduleTrigger:
		set.CronExpression, set.Timezone = &s.CronExpression, &s.Timezone
	case WebhookTrigger:
		set.SigningMode, set.ReplayWindowSec = &s.SigningMode, &s.ReplayWindowSec
	}
	if err := t.Change(r, set, now); err != nil {
		return RoutineTrigger{}, err
	}
	t.Reschedule(r, now)
	return t, nil
}
