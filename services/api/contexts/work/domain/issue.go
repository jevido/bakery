package domain

import (
	"errors"
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
// AssigneeAgentID, ProjectID, ApplicationID, GoalID and ParentID are 0 for
// none, and
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
	// ApplicationID is the Issue's Application: one Application of its
	// Project, which a Run on the Issue works in.
	ApplicationID uint64
	GoalID        uint64
	ParentID      uint64
	CreatedBy     Actor
	// CheckoutRunID is the Run holding the Issue's Checkout, 0 for none;
	// CheckedOutAt is when it took it.
	CheckoutRunID uint64
	CheckedOutAt  *time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	CancelledAt   *time.Time
	// OriginRoutineID and OriginRoutineRunID name the Routine and Routine
	// run that created an Execution Issue; 0 for any other Issue.
	OriginRoutineID    uint64
	OriginRoutineRunID uint64
	// Conversation makes the Issue a Conversation; nil for any other
	// Issue. It is never changed in place: a Comment's ConversationMove
	// is stored instead.
	Conversation *Conversation
	CreatedAt    time.Time
	UpdatedAt    time.Time
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
// A Conversation is never done or cancelled.
func (i *Issue) SetStatus(status IssueStatus, now time.Time) error {
	if i.Conversation != nil && (status == Done || status == IssueCancelled) {
		return ErrConversationClosed
	}
	i.setStatus(status, now)
	return nil
}

func (i *Issue) setStatus(status IssueStatus, now time.Time) {
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
// assigned to. A new Assignee ends any Checkout. A Conversation has no
// Member assignee.
func (i *Issue) Assign(memberID uint64) error {
	if i.Conversation != nil && memberID != 0 {
		return fixedForConversation("assignee_id")
	}
	if memberID != i.AssigneeID {
		i.clearCheckout()
	}
	i.AssigneeID = memberID
	if memberID != 0 {
		i.AssigneeAgentID = 0
	}
	return nil
}

// AssignAgent hands the Issue to an Agent, already known to be one of the
// Guild's and not terminated, taking it from a Member; 0 takes it from the
// Agent it is assigned to. A new Assignee ends any Checkout. A
// Conversation's Agent assignee is its Conversation agent, always.
func (i *Issue) AssignAgent(agentID uint64) error {
	if i.Conversation != nil && agentID != i.Conversation.AgentID {
		return fixedForConversation("assignee_agent_id")
	}
	i.assignAgent(agentID)
	return nil
}

func (i *Issue) assignAgent(agentID uint64) {
	if agentID != i.AssigneeAgentID {
		i.clearCheckout()
	}
	i.AssigneeAgentID = agentID
	if agentID != 0 {
		i.AssigneeID = 0
	}
}

func (i *Issue) clearCheckout() {
	i.CheckoutRunID = 0
	i.CheckedOutAt = nil
}

// CheckoutStatuses are the Issue statuses a Checkout takes the Issue from
// when it names none, as Paperclip's MCP server's.
var CheckoutStatuses = []IssueStatus{Todo, Backlog, Blocked}

// ErrNotHolder is a Release by a Run that does not hold the Checkout.
var ErrNotHolder = errors.New("issue is not checked out by this run")

// ErrNotAssignee is a Checkout by an Agent the Issue is not assigned to.
var ErrNotAssignee = errors.New("issue is assigned to someone else")

// HeldError is an Issue whose Checkout another live Run holds.
type HeldError struct{ RunID uint64 }

func (e *HeldError) Error() string { return "checked out by another run" }

// StatusError is a Checkout of an Issue in a status it was not expected
// in.
type StatusError struct{ Status IssueStatus }

func (e *StatusError) Error() string { return fmt.Sprintf("issue status is %s", e.Status) }

// HeldByOther tells whether a Run other than runID holds the Issue's
// Checkout and is live; a Stale checkout holds nothing.
func (i Issue) HeldByOther(runID uint64, live func(runID uint64) bool) bool {
	return i.CheckoutRunID != 0 && i.CheckoutRunID != runID && live(i.CheckoutRunID)
}

// Checkout gives the Issue's Checkout to the Agent's Run at now and moves
// it to in progress. The Issue must be assigned to the Agent, or to nobody
// (then it is assigned to it), and in one of expected (CheckoutStatuses
// when empty), or already in progress for the Agent, as Paperclip adopts
// it. A Run that holds it already changes nothing (changed is false); a
// Stale checkout, whose Run live says is not running, is taken over; one
// held by a live Run is a HeldError.
func (i *Issue) Checkout(agentID, runID uint64, expected []IssueStatus, live func(runID uint64) bool, now time.Time) (changed bool, err error) {
	switch {
	case i.AssigneeAgentID == agentID:
	case i.AssigneeAgentID == 0 && i.AssigneeID == 0:
	default:
		return false, ErrNotAssignee
	}
	if i.CheckoutRunID == runID && i.AssigneeAgentID == agentID {
		return false, nil
	}
	if len(expected) == 0 {
		expected = CheckoutStatuses
	}
	if !slices.Contains(expected, i.Status) && (i.Status != InProgress || i.AssigneeAgentID != agentID) {
		return false, &StatusError{Status: i.Status}
	}
	if i.HeldByOther(runID, live) {
		return false, &HeldError{RunID: i.CheckoutRunID}
	}
	i.assignAgent(agentID)
	t := now.UTC()
	i.CheckoutRunID, i.CheckedOutAt = runID, &t
	i.setStatus(InProgress, now)
	return true, nil
}

// Release gives up the Run's Checkout at now, as Paperclip's: an Issue in
// progress goes back to todo, and an open one loses its Agent assignee,
// so it goes back to the pool. A Conversation keeps its Agent.
func (i *Issue) Release(runID uint64, now time.Time) error {
	if i.CheckoutRunID == 0 || i.CheckoutRunID != runID {
		return ErrNotHolder
	}
	i.clearCheckout()
	if i.Status == InProgress {
		i.setStatus(Todo, now)
	}
	if i.Status != Done && i.Status != IssueCancelled && i.Conversation == nil {
		i.assignAgent(0)
	}
	return nil
}

// PlaceIn puts the Issue in a Project, already known to be one of the
// Guild's; 0 is none. Moving it to another Project lets go of the Issue's
// Application, which belongs to the old one. A Conversation has no
// Project.
func (i *Issue) PlaceIn(projectID uint64) error {
	if i.Conversation != nil && projectID != 0 {
		return fixedForConversation("project_id")
	}
	if projectID != i.ProjectID {
		i.ApplicationID = 0
	}
	i.ProjectID = projectID
	return nil
}

// SetApplication names the Issue's Application, already known to be one
// of its Project's; 0 is none. An Issue without a Project has none.
func (i *Issue) SetApplication(applicationID uint64) error {
	if i.Conversation != nil && applicationID != 0 {
		return fixedForConversation("application_id")
	}
	if applicationID != 0 && i.ProjectID == 0 {
		return invalid("application_id", "the application is not in the issue's project")
	}
	i.ApplicationID = applicationID
	return nil
}

// ServeGoal ties the Issue to a Goal, which must be the Guild's; nil is
// none.
func (i *Issue) ServeGoal(goal *Goal) error {
	if goal == nil {
		i.GoalID = 0
		return nil
	}
	if i.Conversation != nil {
		return fixedForConversation("goal_id")
	}
	if goal.GuildID != i.GuildID {
		return invalid("goal_id", "goal not found")
	}
	i.GoalID = goal.ID
	return nil
}

// MoveUnder makes parent the Issue's parent, nil for none. ancestors are
// the ids of parent's own ancestors, nearest first: the Issue may be none
// of them, nor parent itself. A Conversation has no parent and no
// Sub-issues.
func (i *Issue) MoveUnder(parent *Issue, ancestors []uint64) error {
	if parent == nil {
		i.ParentID = 0
		return nil
	}
	if i.Conversation != nil {
		return fixedForConversation("parent_id")
	}
	if parent.Conversation != nil {
		return invalid("parent_id", "a conversation cannot have sub-issues")
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
// it, nor the Issue itself, nor another Guild's Issue. A Conversation
// neither blocks nor is blocked.
func (i Issue) BlockWith(blockers []Issue, reachable []uint64) ([]uint64, error) {
	if i.Conversation != nil && len(blockers) > 0 {
		return nil, fixedForConversation("blocked_by_ids")
	}
	ids := make([]uint64, 0, len(blockers))
	for _, b := range blockers {
		switch {
		case b.Conversation != nil:
			return nil, invalid("blocked_by_ids", "a conversation cannot block an issue")
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

// AgentBranch is the git branch an Agent pushes for the Issue with the
// identifier, on every Run of it, and the head of its Pull request:
// "bakery/" and the identifier in lower case, so "DEF-12" is
// "bakery/def-12".
func AgentBranch(identifier string) string {
	return "bakery/" + strings.ToLower(identifier)
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
