package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrIssueCycle is a parent Issue that is the Issue itself or one of its
// Sub-issues.
var ErrIssueCycle error = &FieldError{Field: "parent_id", Message: "an issue cannot sit under itself or one of its sub-issues"}

// ErrSelfBlock is an Issue named among its own Blockers.
var ErrSelfBlock error = &FieldError{Field: "blocked_by_ids", Message: "an issue cannot block itself"}

// ErrBlockerCycle is a Blocker that the Issue already blocks, directly or
// through others.
var ErrBlockerCycle error = &FieldError{Field: "blocked_by_ids", Message: "an issue cannot be blocked by an issue it blocks"}

// IssueStatus is where an Issue stands.
type IssueStatus string

const (
	Backlog        IssueStatus = "backlog"
	Todo           IssueStatus = "todo"
	InProgress     IssueStatus = "in_progress"
	InReview       IssueStatus = "in_review"
	Blocked        IssueStatus = "blocked"
	Done           IssueStatus = "done"
	IssueCancelled IssueStatus = "cancelled"
)

// IssueStatuses lists every Issue status in the order an Issue moves
// through them.
var IssueStatuses = []IssueStatus{Backlog, Todo, InProgress, InReview, Blocked, Done, IssueCancelled}

// ParseIssueStatus reads an Issue status by its wire key.
func ParseIssueStatus(s string) (IssueStatus, error) {
	if st := IssueStatus(s); slices.Contains(IssueStatuses, st) {
		return st, nil
	}
	return "", invalid("status", "status must be backlog, todo, in_progress, in_review, blocked, done or cancelled")
}

// Priority is how urgent an Issue is.
type Priority string

const (
	Critical Priority = "critical"
	High     Priority = "high"
	Medium   Priority = "medium"
	Low      Priority = "low"
)

// Priorities lists every Priority, most urgent first.
var Priorities = []Priority{Critical, High, Medium, Low}

// ParsePriority reads a Priority by its wire key.
func ParsePriority(s string) (Priority, error) {
	if p := Priority(s); slices.Contains(Priorities, p) {
		return p, nil
	}
	return "", invalid("priority", "priority must be critical, high, medium or low")
}

// Issue is one piece of work in a Guild. Its Assignee is a Member
// (AssigneeID) or an Agent (AssigneeAgentID), never both. AssigneeID,
// AssigneeAgentID, ProjectID, GoalID and ParentID are 0 for none, and
// CreatedBy is the Member or Agent that created it; the times are nil until the
// Issue status sets them.
type Issue struct {
	ID          uint64
	GuildID     uint64
	Number      int
	Title       string
	Description string
	Status      IssueStatus
	Priority    Priority
	AssigneeID  uint64
	// AssigneeAgentID is the Agent the Issue is assigned to.
	AssigneeAgentID uint64
	ProjectID       uint64
	GoalID          uint64
	ParentID        uint64
	CreatedBy       Actor
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CancelledAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewIssue is an Issue of the Guild, created by a Member or an Agent, in the Backlog
// at medium Priority until told otherwise. Its number is given when it is
// stored.
func NewIssue(guildID uint64, createdBy Actor, title, description string) (Issue, error) {
	i := Issue{GuildID: guildID, CreatedBy: createdBy, Description: description, Status: Backlog, Priority: Medium}
	if err := i.Rename(title); err != nil {
		return Issue{}, err
	}
	return i, nil
}

func (i *Issue) Rename(title string) error {
	t, err := Title("title", title)
	if err != nil {
		return err
	}
	i.Title = t
	return nil
}

// Describe sets the Markdown description; empty is none.
func (i *Issue) Describe(description string) { i.Description = description }

// SetStatus moves the Issue to status at now: in_progress sets the started
// time once, done the completed time and cancelled the cancelled time;
// leaving done or cancelled clears that time again.
func (i *Issue) SetStatus(status IssueStatus, now time.Time) {
	if status == i.Status {
		return
	}
	switch i.Status {
	case Done:
		i.CompletedAt = nil
	case IssueCancelled:
		i.CancelledAt = nil
	}
	t := now.UTC()
	switch status {
	case InProgress:
		if i.StartedAt == nil {
			i.StartedAt = &t
		}
	case Done:
		i.CompletedAt = &t
	case IssueCancelled:
		i.CancelledAt = &t
	}
	i.Status = status
}

func (i *Issue) SetPriority(p Priority) { i.Priority = p }

// Assign hands the Issue to a Member, already known to be one of the
// Guild's, taking it from an Agent; 0 takes it from the Member it is
// assigned to.
func (i *Issue) Assign(memberID uint64) {
	i.AssigneeID = memberID
	if memberID != 0 {
		i.AssigneeAgentID = 0
	}
}

// AssignAgent hands the Issue to an Agent, already known to be one of the
// Guild's and not terminated, taking it from a Member; 0 takes it from the
// Agent it is assigned to.
func (i *Issue) AssignAgent(agentID uint64) {
	i.AssigneeAgentID = agentID
	if agentID != 0 {
		i.AssigneeID = 0
	}
}

// PlaceIn puts the Issue in a Project, already known to be one of the
// Guild's; 0 is none.
func (i *Issue) PlaceIn(projectID uint64) { i.ProjectID = projectID }

// ServeGoal ties the Issue to a Goal, which must be the Guild's; nil is
// none.
func (i *Issue) ServeGoal(goal *Goal) error {
	if goal == nil {
		i.GoalID = 0
		return nil
	}
	if goal.GuildID != i.GuildID {
		return invalid("goal_id", "goal not found")
	}
	i.GoalID = goal.ID
	return nil
}

// MoveUnder makes parent the Issue's parent, nil for none. ancestors are
// the ids of parent's own ancestors, nearest first: the Issue may be none
// of them, nor parent itself.
func (i *Issue) MoveUnder(parent *Issue, ancestors []uint64) error {
	if parent == nil {
		i.ParentID = 0
		return nil
	}
	if parent.GuildID != i.GuildID {
		return invalid("parent_id", "parent issue not found")
	}
	if i.ID != 0 && (parent.ID == i.ID || slices.Contains(ancestors, i.ID)) {
		return ErrIssueCycle
	}
	i.ParentID = parent.ID
	return nil
}

// BlockWith checks blockers as the Issue's Blockers and returns their ids
// in order, without repeats. reachable are the ids of every Issue the
// Issue already blocks, directly or through others: none of them may block
// it, nor the Issue itself, nor another Guild's Issue.
func (i Issue) BlockWith(blockers []Issue, reachable []uint64) ([]uint64, error) {
	ids := make([]uint64, 0, len(blockers))
	for _, b := range blockers {
		switch {
		case b.GuildID != i.GuildID:
			return nil, invalid("blocked_by_ids", "blocked-by issue not found")
		case b.ID == i.ID:
			return nil, ErrSelfBlock
		case slices.Contains(reachable, b.ID):
			return nil, ErrBlockerCycle
		}
		if !slices.Contains(ids, b.ID) {
			ids = append(ids, b.ID)
		}
	}
	return ids, nil
}

// Identifier is an Issue identifier, e.g. DEF-12.
func Identifier(prefix string, number int) string {
	return fmt.Sprintf("%s-%d", prefix, number)
}

// ParseIdentifier splits an Issue identifier on its last "-" into its
// Issue prefix, uppercased, and its number.
func ParseIdentifier(s string) (prefix string, number int, ok bool) {
	at := strings.LastIndex(s, "-")
	if at <= 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(s[at+1:])
	if err != nil || n < 1 {
		return "", 0, false
	}
	return strings.ToUpper(s[:at]), n, true
}
