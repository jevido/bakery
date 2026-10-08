package domain

import (
	"slices"
	"time"
)

// Actions: the dotted names of what happened to a Goal, an Issue, an
// Approval or an Agent, as Paperclip names them.
const (
	GoalCreatedAction          = "goal.created"
	GoalUpdatedAction          = "goal.updated"
	GoalDeletedAction          = "goal.deleted"
	IssueCreatedAction         = "issue.created"
	IssueUpdatedAction         = "issue.updated"
	IssueDeletedAction         = "issue.deleted"
	CommentAddedAction         = "issue.comment_added"
	CommentDeletedAction       = "issue.comment_deleted"
	DocumentCreatedAction      = "issue.document_created"
	DocumentUpdatedAction      = "issue.document_updated"
	DocumentDeletedAction      = "issue.document_deleted"
	ApprovalCreatedAction      = "approval.created"
	ApprovalApprovedAction     = "approval.approved"
	ApprovalRejectedAction     = "approval.rejected"
	RevisionRequestedAction    = "approval.revision_requested"
	ApprovalResubmittedAction  = "approval.resubmitted"
	ApprovalCommentAddedAction = "approval.comment_added"
	ApprovalCancelledAction    = "approval.cancelled"
	AgentHiredAction           = "agent.hired"
	AgentUpdatedAction         = "agent.updated"
	AgentPausedAction          = "agent.paused"
	AgentResumedAction         = "agent.resumed"
	AgentTerminatedAction      = "agent.terminated"
	AgentRoleAddedAction       = "agent.role_added"
	AgentRoleRemovedAction     = "agent.role_removed"
	RunStartedAction           = "run.started"
	RunFinishedAction          = "run.finished"
)

// AgentActions lists the Actions the agents context records through work.
var AgentActions = []string{AgentHiredAction, AgentUpdatedAction, AgentPausedAction, AgentResumedAction, AgentTerminatedAction, AgentRoleAddedAction, AgentRoleRemovedAction, RunStartedAction, RunFinishedAction}

// The kinds of thing an Activity event is about.
const (
	IssueEntity    = "issue"
	GoalEntity     = "goal"
	ApprovalEntity = "approval"
	AgentEntity    = "agent"
)

// SnippetLength is how many characters of a Comment its Activity event
// keeps, as Paperclip's bodySnippet.
const SnippetLength = 140

// ActivityEvent is one entry in the Guild's Activity: an Actor did an
// Action to one Goal, Issue, Approval or Agent at a time, with what changed. ID is 0 until
// it is recorded; ProjectID is 0 for none and the Actor is nobody for none.
type ActivityEvent struct {
	ID         uint64
	GuildID    uint64
	Actor      Actor
	Action     string
	EntityType string
	EntityID   uint64
	ProjectID  uint64
	Details    map[string]any
	CreatedAt  time.Time
}

// Event is one of work's domain events; each is recorded as one Activity
// event.
type Event interface {
	Activity() ActivityEvent
}

// Happened is who caused a domain event and when.
type Happened struct {
	Actor Actor
	At    time.Time
}

func (h Happened) goal(g Goal, action string, details map[string]any) ActivityEvent {
	details["title"] = g.Title
	return ActivityEvent{GuildID: g.GuildID, Actor: h.Actor, Action: action, EntityType: GoalEntity, EntityID: g.ID, Details: details, CreatedAt: h.At}
}

// issue keeps the Issue's number and title in every event about it, so
// one about a deleted Issue still reads.
func (h Happened) issue(i Issue, action string, details map[string]any) ActivityEvent {
	details["issue_number"], details["issue_title"] = i.Number, i.Title
	return ActivityEvent{GuildID: i.GuildID, Actor: h.Actor, Action: action, EntityType: IssueEntity, EntityID: i.ID, ProjectID: i.ProjectID, Details: details, CreatedAt: h.At}
}

// approval keeps the Approval's type and payload title in every event
// about it, so one still reads after the title changes. An Approval has no
// Project.
func (h Happened) approval(a Approval, action string, details map[string]any) ActivityEvent {
	details["type"], details["title"] = a.Type, a.Payload.Label()
	return ActivityEvent{GuildID: a.GuildID, Actor: h.Actor, Action: action, EntityType: ApprovalEntity, EntityID: a.ID, Details: details, CreatedAt: h.At}
}

// snippet is the first SnippetLength characters of a comment's body.
func snippet(body string) string {
	r := []rune(body)
	if len(r) > SnippetLength {
		r = r[:SnippetLength]
	}
	return string(r)
}

// change is one field's from → to; a reference is its id, null for none.
func change(from, to any) map[string]any { return map[string]any{"from": from, "to": to} }

func ref(id uint64) any {
	if id == 0 {
		return nil
	}
	return id
}

// assignee is the Issue's Assignee as an Activity event keeps it: its id
// and kind, member or agent; nil for none. Events from before Agents could
// be Assignees keep a Member's bare id.
func assignee(i Issue) any {
	switch {
	case i.AssigneeAgentID != 0:
		return map[string]any{"id": i.AssigneeAgentID, "kind": "agent"}
	case i.AssigneeID != 0:
		return map[string]any{"id": i.AssigneeID, "kind": "member"}
	}
	return nil
}

type GoalCreated struct {
	Happened
	Goal Goal
}

func (e GoalCreated) Activity() ActivityEvent {
	return e.goal(e.Goal, GoalCreatedAction, map[string]any{"level": e.Goal.Level, "status": e.Goal.Status})
}

// GoalChanged is a Goal as it was before a change and as it was stored.
type GoalChanged struct {
	Happened
	Before, After Goal
}

// Changes is every field that differs, from → to; empty when nothing did.
func (e GoalChanged) Changes() map[string]any {
	b, a := e.Before, e.After
	out := map[string]any{}
	if b.Title != a.Title {
		out["title"] = change(b.Title, a.Title)
	}
	if b.Description != a.Description {
		out["description"] = true
	}
	if b.Level != a.Level {
		out["level"] = change(b.Level, a.Level)
	}
	if b.Status != a.Status {
		out["status"] = change(b.Status, a.Status)
	}
	if b.ParentID != a.ParentID {
		out["parent"] = change(ref(b.ParentID), ref(a.ParentID))
	}
	if b.OwnerID != a.OwnerID {
		out["owner"] = change(ref(b.OwnerID), ref(a.OwnerID))
	}
	return out
}

func (e GoalChanged) Activity() ActivityEvent {
	return e.goal(e.After, GoalUpdatedAction, map[string]any{"changes": e.Changes()})
}

type GoalDeleted struct {
	Happened
	Goal Goal
}

func (e GoalDeleted) Activity() ActivityEvent {
	return e.goal(e.Goal, GoalDeletedAction, map[string]any{})
}

type IssueCreated struct {
	Happened
	Issue Issue
}

func (e IssueCreated) Activity() ActivityEvent {
	return e.issue(e.Issue, IssueCreatedAction, map[string]any{"status": e.Issue.Status, "priority": e.Issue.Priority})
}

// IssueChanged is an Issue as it was before a change and as it was
// stored, with the Blockers the person could see before and after it;
// nil Blockers when the change did not set them.
type IssueChanged struct {
	Happened
	Before, After                 Issue
	BlockersBefore, BlockersAfter []uint64
}

// Changes is every field that differs, from → to, and the Blockers added
// and removed; empty when nothing did.
func (e IssueChanged) Changes() map[string]any {
	b, a := e.Before, e.After
	out := map[string]any{}
	if b.Title != a.Title {
		out["title"] = change(b.Title, a.Title)
	}
	if b.Description != a.Description {
		out["description"] = true
	}
	if b.Status != a.Status {
		out["status"] = change(b.Status, a.Status)
	}
	if b.Priority != a.Priority {
		out["priority"] = change(b.Priority, a.Priority)
	}
	if b.AssigneeID != a.AssigneeID || b.AssigneeAgentID != a.AssigneeAgentID {
		out["assignee"] = change(assignee(b), assignee(a))
	}
	for _, f := range []struct {
		name     string
		from, to uint64
	}{
		{"project", b.ProjectID, a.ProjectID},
		{"goal", b.GoalID, a.GoalID},
		{"parent", b.ParentID, a.ParentID},
	} {
		if f.from != f.to {
			out[f.name] = change(ref(f.from), ref(f.to))
		}
	}
	added, removed := diffIDs(e.BlockersBefore, e.BlockersAfter), diffIDs(e.BlockersAfter, e.BlockersBefore)
	if len(added) > 0 || len(removed) > 0 {
		out["blockers"] = map[string]any{"added": added, "removed": removed}
	}
	return out
}

// diffIDs is the ids in b that are not in a, sorted.
func diffIDs(a, b []uint64) []uint64 {
	out := []uint64{}
	for _, id := range b {
		if !slices.Contains(a, id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func (e IssueChanged) Activity() ActivityEvent {
	return e.issue(e.After, IssueUpdatedAction, map[string]any{"changes": e.Changes()})
}

type IssueDeleted struct {
	Happened
	Issue Issue
}

func (e IssueDeleted) Activity() ActivityEvent {
	return e.issue(e.Issue, IssueDeletedAction, map[string]any{})
}

type CommentWritten struct {
	Happened
	Issue   Issue
	Comment Comment
}

func (e CommentWritten) Activity() ActivityEvent {
	return e.issue(e.Issue, CommentAddedAction, map[string]any{"comment_id": e.Comment.ID, "snippet": snippet(e.Comment.Body)})
}

type CommentDeleted struct {
	Happened
	Issue   Issue
	Comment Comment
}

func (e CommentDeleted) Activity() ActivityEvent {
	return e.issue(e.Issue, CommentDeletedAction, map[string]any{"comment_id": e.Comment.ID})
}

// DocumentSaved is an Issue document saved as a new Revision: its first
// save (First), a later one, or a Restore of Revision RestoredFrom (0 for
// none).
type DocumentSaved struct {
	Happened
	Issue        Issue
	Document     IssueDocument
	First        bool
	RestoredFrom int
}

func (e DocumentSaved) Activity() ActivityEvent {
	action := DocumentUpdatedAction
	if e.First {
		action = DocumentCreatedAction
	}
	details := map[string]any{"key": e.Document.Key, "title": e.Document.Title, "revision_number": e.Document.Latest}
	if e.RestoredFrom != 0 {
		details["restored_from"] = e.RestoredFrom
	}
	return e.issue(e.Issue, action, details)
}

type DocumentDeleted struct {
	Happened
	Issue    Issue
	Document IssueDocument
}

func (e DocumentDeleted) Activity() ActivityEvent {
	return e.issue(e.Issue, DocumentDeletedAction, map[string]any{"key": e.Document.Key, "title": e.Document.Title})
}

type ApprovalRequested struct {
	Happened
	Approval Approval
}

func (e ApprovalRequested) Activity() ActivityEvent {
	return e.approval(e.Approval, ApprovalCreatedAction, map[string]any{"issue_ids": e.Approval.IssueIDs})
}

// decided is the details of a Decision: its note, when there is one.
func decided(a Approval) map[string]any {
	d := map[string]any{}
	if a.DecisionNote != "" {
		d["decision_note"] = a.DecisionNote
	}
	return d
}

type ApprovalApproved struct {
	Happened
	Approval Approval
}

func (e ApprovalApproved) Activity() ActivityEvent {
	return e.approval(e.Approval, ApprovalApprovedAction, decided(e.Approval))
}

type ApprovalRejected struct {
	Happened
	Approval Approval
}

func (e ApprovalRejected) Activity() ActivityEvent {
	return e.approval(e.Approval, ApprovalRejectedAction, decided(e.Approval))
}

type RevisionRequested struct {
	Happened
	Approval Approval
}

func (e RevisionRequested) Activity() ActivityEvent {
	return e.approval(e.Approval, RevisionRequestedAction, decided(e.Approval))
}

type ApprovalResubmitted struct {
	Happened
	Approval Approval
}

func (e ApprovalResubmitted) Activity() ActivityEvent {
	return e.approval(e.Approval, ApprovalResubmittedAction, map[string]any{})
}

type ApprovalCommentWritten struct {
	Happened
	Approval Approval
	Comment  ApprovalComment
}

func (e ApprovalCommentWritten) Activity() ActivityEvent {
	return e.approval(e.Approval, ApprovalCommentAddedAction, map[string]any{"comment_id": e.Comment.ID, "snippet": snippet(e.Comment.Body)})
}

type ApprovalCancelled struct {
	Happened
	Approval Approval
}

func (e ApprovalCancelled) Activity() ActivityEvent {
	return e.approval(e.Approval, ApprovalCancelledAction, map[string]any{})
}

// AgentEvent is something the agents context did to an Agent, recorded
// as an Activity event about it: the Agent's name is kept in its details
// so the event still reads once the Agent is renamed.
type AgentEvent struct {
	Happened
	GuildID   uint64
	AgentID   uint64
	AgentName string
	Action    string
	Details   map[string]any
}

// Validated checks that the event is about an Agent, with a name and an
// Action the agents context records.
func (e AgentEvent) Validated() (AgentEvent, error) {
	if e.GuildID == 0 || e.AgentID == 0 {
		return AgentEvent{}, invalid("agent_id", "an agent event needs its guild and agent")
	}
	if !slices.Contains(AgentActions, e.Action) {
		return AgentEvent{}, invalid("action", "%q is not an agent action", e.Action)
	}
	if e.AgentName == "" {
		return AgentEvent{}, invalid("name", "an agent event needs the agent's name")
	}
	return e, nil
}

func (e AgentEvent) Activity() ActivityEvent {
	details := make(map[string]any, len(e.Details)+1)
	for k, v := range e.Details {
		details[k] = v
	}
	details["name"] = e.AgentName
	return ActivityEvent{GuildID: e.GuildID, Actor: e.Actor, Action: e.Action, EntityType: AgentEntity, EntityID: e.AgentID, Details: details, CreatedAt: e.At}
}
