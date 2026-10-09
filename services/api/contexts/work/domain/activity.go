package domain

import (
	"reflect"
	"slices"
	"time"
)

// Actions: the dotted names of what happened to a Goal, an Issue, an
// Approval or an Agent, as Paperclip names them.
const (
	GoalCreatedAction     = "goal.created"
	GoalUpdatedAction     = "goal.updated"
	GoalDeletedAction     = "goal.deleted"
	IssueCreatedAction    = "issue.created"
	IssueUpdatedAction    = "issue.updated"
	IssueDeletedAction    = "issue.deleted"
	IssueCheckedOutAction = "issue.checked_out"
	IssueReleasedAction   = "issue.released"
	// IssueConversationOpenedAction is a Member's Conversation with an
	// Agent, opened.
	IssueConversationOpenedAction = "issue.conversation_opened"
	// IssueApplicationChangedAction is not Paperclip's: its Issues name
	// no Application.
	IssueApplicationChangedAction = "issue.application_changed"
	// The pull request and preview Actions are not Paperclip's: its Work
	// products record no Activity.
	PullRequestOpenedAction           = "issue.pull_request_opened"
	PullRequestMergedAction           = "issue.pull_request_merged"
	PullRequestClosedAction           = "issue.pull_request_closed"
	PreviewReadyAction                = "issue.preview_ready"
	PreviewFailedAction               = "issue.preview_failed"
	CommentAddedAction                = "issue.comment_added"
	CommentDeletedAction              = "issue.comment_deleted"
	DocumentCreatedAction             = "issue.document_created"
	DocumentUpdatedAction             = "issue.document_updated"
	DocumentDeletedAction             = "issue.document_deleted"
	ApprovalCreatedAction             = "approval.created"
	ApprovalApprovedAction            = "approval.approved"
	ApprovalRejectedAction            = "approval.rejected"
	RevisionRequestedAction           = "approval.revision_requested"
	ApprovalResubmittedAction         = "approval.resubmitted"
	ApprovalCommentAddedAction        = "approval.comment_added"
	ApprovalCancelledAction           = "approval.cancelled"
	AgentHiredAction                  = "agent.hired"
	AgentUpdatedAction                = "agent.updated"
	AgentPausedAction                 = "agent.paused"
	AgentResumedAction                = "agent.resumed"
	AgentTerminatedAction             = "agent.terminated"
	AgentRoleAddedAction              = "agent.role_added"
	AgentRoleRemovedAction            = "agent.role_removed"
	RunStartedAction                  = "run.started"
	RunFinishedAction                 = "run.finished"
	AgentSkillsSyncedAction           = "agent.skills_synced"
	SkillCreatedAction                = "skill.created"
	SkillFileUpdatedAction            = "skill.file_updated"
	SkillFileDeletedAction            = "skill.file_deleted"
	SkillDeletedAction                = "skill.deleted"
	BudgetUpdatedAction               = "budget.updated"
	BudgetSoftCrossedAction           = "budget.soft_threshold_crossed"
	BudgetHardCrossedAction           = "budget.hard_threshold_crossed"
	BudgetIncidentResolved            = "budget.incident_resolved"
	RoutineCreatedAction              = "routine.created"
	RoutineUpdatedAction              = "routine.updated"
	RoutineArchivedAction             = "routine.archived"
	RoutineTriggerCreatedAction       = "routine.trigger_created"
	RoutineTriggerUpdatedAction       = "routine.trigger_updated"
	RoutineTriggerDeletedAction       = "routine.trigger_deleted"
	RoutineTriggerSecretRotatedAction = "routine.trigger_secret_rotated"
	RoutineRevisionRestoredAction     = "routine.revision_restored"
	RoutineRunTriggeredAction         = "routine.run_triggered"
	RoutineWebhookRejectedAction      = "routine.webhook_rejected"
)

// AgentActions lists the Actions the agents context records through work.
var AgentActions = []string{AgentHiredAction, AgentUpdatedAction, AgentPausedAction, AgentResumedAction, AgentTerminatedAction, AgentRoleAddedAction, AgentRoleRemovedAction, RunStartedAction, RunFinishedAction, AgentSkillsSyncedAction}

// The kinds of thing an Activity event is about.
const (
	IssueEntity    = "issue"
	GoalEntity     = "goal"
	ApprovalEntity = "approval"
	AgentEntity    = "agent"
	BudgetEntity   = "budget"
	IncidentEntity = "budget_incident"
	RoutineEntity  = "routine"
	SkillEntity    = "skill"
)

// EntityActions lists the Actions about a Budget, a Budget incident or a
// Skill the agents context records through work, with what each is about.
var EntityActions = map[string]string{
	BudgetUpdatedAction: BudgetEntity, BudgetSoftCrossedAction: IncidentEntity,
	BudgetHardCrossedAction: IncidentEntity, BudgetIncidentResolved: IncidentEntity,
	SkillCreatedAction: SkillEntity, SkillFileUpdatedAction: SkillEntity,
	SkillFileDeletedAction: SkillEntity, SkillDeletedAction: SkillEntity,
}

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

// routine keeps the Routine's title in every event about it, and its
// Project, so its events stay hidden where it is.
func (h Happened) routine(r Routine, action string, details map[string]any) ActivityEvent {
	details["title"] = r.Title
	return ActivityEvent{GuildID: r.GuildID, Actor: h.Actor, Action: action, EntityType: RoutineEntity, EntityID: r.ID, ProjectID: r.ProjectID, Details: details, CreatedAt: h.At}
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

// ConversationOpened is a Conversation, created; AgentName keeps the
// Conversation agent's name so the event still reads after it changes.
type ConversationOpened struct {
	Happened
	Issue     Issue
	AgentName string
}

func (e ConversationOpened) Activity() ActivityEvent {
	return e.issue(e.Issue, IssueConversationOpenedAction, map[string]any{"agent_id": e.Issue.Conversation.AgentID, "agent_name": e.AgentName})
}

// IssueApplicationChanged is the Issue's Application named, changed or
// let go of. From and To are Applications by name, nil for none; the
// names are kept, so the event still reads after one is deleted.
type IssueApplicationChanged struct {
	Happened
	Issue    Issue
	From, To *NamedApplication
}

// NamedApplication is an Application of a Project as work knows it: its
// id and name.
type NamedApplication struct {
	ID   uint64
	Name string
}

func (a *NamedApplication) details() any {
	if a == nil {
		return nil
	}
	return map[string]any{"id": a.ID, "name": a.Name}
}

func (e IssueApplicationChanged) Activity() ActivityEvent {
	return e.issue(e.Issue, IssueApplicationChangedAction, map[string]any{"from": e.From.details(), "to": e.To.details()})
}

// IssueCheckedOut is a Run taking an Issue's Checkout.
type IssueCheckedOut struct {
	Happened
	Issue Issue
	RunID uint64
}

func (e IssueCheckedOut) Activity() ActivityEvent {
	return e.issue(e.Issue, IssueCheckedOutAction, map[string]any{"run_id": e.RunID})
}

// IssueReleased is a Run giving up an Issue's Checkout.
type IssueReleased struct {
	Happened
	Issue Issue
	RunID uint64
}

func (e IssueReleased) Activity() ActivityEvent {
	return e.issue(e.Issue, IssueReleasedAction, map[string]any{"run_id": e.RunID})
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
// so the event still reads once the Agent is renamed. With EntityID set it
// is about that Budget, Budget incident or Skill instead (EntityActions
// tells which), and AgentName is the Budget scope's or the Skill's name.
type AgentEvent struct {
	Happened
	GuildID   uint64
	AgentID   uint64
	EntityID  uint64
	AgentName string
	Action    string
	Details   map[string]any
}

// Validated checks that the event is about an Agent (or a Budget or Budget
// incident), with a name and an Action the agents context records for it.
func (e AgentEvent) Validated() (AgentEvent, error) {
	if e.EntityID != 0 {
		if _, ok := EntityActions[e.Action]; !ok || e.GuildID == 0 {
			return AgentEvent{}, invalid("action", "%q is not a budget or skill action", e.Action)
		}
	} else if e.GuildID == 0 || e.AgentID == 0 {
		return AgentEvent{}, invalid("agent_id", "an agent event needs its guild and agent")
	} else if !slices.Contains(AgentActions, e.Action) {
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
	if e.EntityID != 0 {
		// A Project's Budget is in its Project, so its events are hidden
		// from whoever may not view it.
		var projectID uint64
		if details["scope_type"] == "project" {
			projectID, _ = details["scope_id"].(uint64)
		}
		return ActivityEvent{GuildID: e.GuildID, Actor: e.Actor, Action: e.Action, EntityType: EntityActions[e.Action], EntityID: e.EntityID, ProjectID: projectID, Details: details, CreatedAt: e.At}
	}
	return ActivityEvent{GuildID: e.GuildID, Actor: e.Actor, Action: e.Action, EntityType: AgentEntity, EntityID: e.AgentID, Details: details, CreatedAt: e.At}
}

// PullRequestOpened is a Pull request recorded as one of an Issue's Work
// products.
type PullRequestOpened struct {
	Happened
	Issue       Issue
	WorkProduct WorkProduct
}

func (e PullRequestOpened) Activity() ActivityEvent {
	w := e.WorkProduct
	return e.issue(e.Issue, PullRequestOpenedAction, map[string]any{"provider": w.Provider, "number": w.ExternalID, "url": w.URL})
}

// WorkProductMoved is an Issue's Work product moved by what happened on
// the git host or to its Preview: a Pull request merged or closed, a
// Preview ready or failed. Other moves record no Activity.
type WorkProductMoved struct {
	Happened
	Issue       Issue
	WorkProduct WorkProduct
}

// Recorded is false for a move that records no Activity.
func (e WorkProductMoved) Recorded() bool { return e.action() != "" }

func (e WorkProductMoved) action() string {
	return map[WorkProductStatus]string{
		PullRequestMerged: PullRequestMergedAction, PullRequestClosed: PullRequestClosedAction,
		PreviewReady: PreviewReadyAction, PreviewFailed: PreviewFailedAction,
	}[e.WorkProduct.Status]
}

func (e WorkProductMoved) Activity() ActivityEvent {
	w := e.WorkProduct
	details := map[string]any{"number": w.ExternalID, "url": w.URL}
	if w.Type == PullRequestProduct {
		details["provider"] = w.Provider
	}
	return e.issue(e.Issue, e.action(), details)
}

type RoutineCreated struct {
	Happened
	Routine Routine
}

func (e RoutineCreated) Activity() ActivityEvent {
	return e.routine(e.Routine, RoutineCreatedAction, map[string]any{"status": e.Routine.Status})
}

// RoutineChanged is a Routine as it was before a change and as it was
// stored, when the change did not archive it.
type RoutineChanged struct {
	Happened
	Before, After Routine
}

// Changes is every field that differs, from → to; empty when nothing did.
func (e RoutineChanged) Changes() map[string]any {
	b, a := e.Before, e.After
	out := map[string]any{}
	if b.Title != a.Title {
		out["title"] = change(b.Title, a.Title)
	}
	if b.Description != a.Description {
		out["description"] = true
	}
	if bv, av := variablesRef(b.Variables), variablesRef(a.Variables); !reflect.DeepEqual(b.Variables, a.Variables) {
		out["variables"] = change(bv, av)
	}
	for _, f := range []struct {
		name     string
		from, to any
		differ   bool
	}{
		{"priority", b.Priority, a.Priority, b.Priority != a.Priority},
		{"status", b.Status, a.Status, b.Status != a.Status},
		{"concurrency_policy", b.ConcurrencyPolicy, a.ConcurrencyPolicy, b.ConcurrencyPolicy != a.ConcurrencyPolicy},
		{"catch_up_policy", b.CatchUpPolicy, a.CatchUpPolicy, b.CatchUpPolicy != a.CatchUpPolicy},
		{"project", ref(b.ProjectID), ref(a.ProjectID), b.ProjectID != a.ProjectID},
		{"goal", ref(b.GoalID), ref(a.GoalID), b.GoalID != a.GoalID},
		{"parent", ref(b.ParentIssueID), ref(a.ParentIssueID), b.ParentIssueID != a.ParentIssueID},
		{"assignee", agentRef(b.AssigneeAgentID), agentRef(a.AssigneeAgentID), b.AssigneeAgentID != a.AssigneeAgentID},
	} {
		if f.differ {
			out[f.name] = change(f.from, f.to)
		}
	}
	return out
}

func (e RoutineChanged) Activity() ActivityEvent {
	return e.routine(e.After, RoutineUpdatedAction, map[string]any{"changes": e.Changes()})
}

// variablesRef is a Routine's variables as an Activity event keeps them:
// each one's name and type.
func variablesRef(vars []RoutineVariable) []map[string]any {
	out := make([]map[string]any, len(vars))
	for n, v := range vars {
		out[n] = map[string]any{"name": v.Name, "type": string(v.Type)}
	}
	return out
}

// agentRef is an Agent assignee as an Activity event keeps it, as
// assignee does for an Issue's; nil for none.
func agentRef(id uint64) any {
	if id == 0 {
		return nil
	}
	return map[string]any{"id": id, "kind": "agent"}
}

type RoutineArchived struct {
	Happened
	Routine Routine
}

func (e RoutineArchived) Activity() ActivityEvent {
	return e.routine(e.Routine, RoutineArchivedAction, map[string]any{})
}

// triggerDetails is what every event about a Routine trigger keeps of it.
func triggerDetails(t RoutineTrigger) map[string]any {
	return map[string]any{"trigger_id": t.ID, "kind": t.Kind, "label": t.Label}
}

type RoutineTriggerAdded struct {
	Happened
	Routine Routine
	Trigger RoutineTrigger
}

func (e RoutineTriggerAdded) Activity() ActivityEvent {
	d := triggerDetails(e.Trigger)
	if e.Trigger.Kind == ScheduleTrigger {
		d["cron_expression"], d["timezone"] = e.Trigger.CronExpression, e.Trigger.Timezone
	}
	if e.Trigger.Kind == WebhookTrigger {
		d["signing_mode"], d["replay_window_sec"] = e.Trigger.SigningMode, e.Trigger.ReplayWindowSec
	}
	return e.routine(e.Routine, RoutineTriggerCreatedAction, d)
}

// RoutineTriggerChanged is a Routine trigger as it was before a change
// and after it.
type RoutineTriggerChanged struct {
	Happened
	Routine       Routine
	Before, After RoutineTrigger
}

// Changes is every field that differs, from → to; empty when nothing did.
func (e RoutineTriggerChanged) Changes() map[string]any {
	b, a := e.Before, e.After
	out := map[string]any{}
	for _, f := range []struct {
		name     string
		from, to any
		differ   bool
	}{
		{"label", b.Label, a.Label, b.Label != a.Label},
		{"cron_expression", b.CronExpression, a.CronExpression, b.CronExpression != a.CronExpression},
		{"timezone", b.Timezone, a.Timezone, b.Timezone != a.Timezone},
		{"enabled", b.Enabled, a.Enabled, b.Enabled != a.Enabled},
		{"signing_mode", b.SigningMode, a.SigningMode, b.SigningMode != a.SigningMode},
		{"replay_window_sec", b.ReplayWindowSec, a.ReplayWindowSec, b.ReplayWindowSec != a.ReplayWindowSec},
	} {
		if f.differ {
			out[f.name] = change(f.from, f.to)
		}
	}
	return out
}

func (e RoutineTriggerChanged) Activity() ActivityEvent {
	d := triggerDetails(e.After)
	d["changes"] = e.Changes()
	return e.routine(e.Routine, RoutineTriggerUpdatedAction, d)
}

type RoutineTriggerDeleted struct {
	Happened
	Routine Routine
	Trigger RoutineTrigger
}

func (e RoutineTriggerDeleted) Activity() ActivityEvent {
	return e.routine(e.Routine, RoutineTriggerDeletedAction, triggerDetails(e.Trigger))
}

// RoutineTriggerSecretRotated is a Webhook trigger given a new secret; the
// secret itself is never recorded.
type RoutineTriggerSecretRotated struct {
	Happened
	Routine Routine
	Trigger RoutineTrigger
}

func (e RoutineTriggerSecretRotated) Activity() ActivityEvent {
	return e.routine(e.Routine, RoutineTriggerSecretRotatedAction, triggerDetails(e.Trigger))
}

// RoutineRevisionRestored is a Routine put back as an older Routine
// revision has it, kept as the new Revision.
type RoutineRevisionRestored struct {
	Happened
	Routine      Routine
	Revision     RoutineRevision
	RestoredFrom RoutineRevision
}

func (e RoutineRevisionRestored) Activity() ActivityEvent {
	return e.routine(e.Routine, RoutineRevisionRestoredAction, map[string]any{
		"revision_id": e.Revision.ID, "revision_number": e.Revision.Number,
		"restored_from_revision_id": e.RestoredFrom.ID, "restored_from_revision_number": e.RestoredFrom.Number,
		"trigger_count": len(e.Revision.Snapshot.Triggers),
	})
}

// RoutineRunTriggered is a Routine run its Schedule or the API made, once
// it is dispatched; a manual one shows as the Issue it created instead.
type RoutineRunTriggered struct {
	Happened
	Routine Routine
	Run     RoutineRun
}

func (e RoutineRunTriggered) Activity() ActivityEvent {
	d := map[string]any{"routine_run_id": e.Run.ID, "source": e.Run.Source, "status": e.Run.Status, "trigger_id": ref(e.Run.TriggerID), "issue": ref(e.Run.LinkedIssueID)}
	return e.routine(e.Routine, RoutineRunTriggeredAction, d)
}

// WebhookDeliveryRejected is a Webhook delivery its Webhook trigger
// refused, for the reason; nobody is its Actor.
type WebhookDeliveryRejected struct {
	Happened
	Routine Routine
	Trigger RoutineTrigger
	Reason  string
}

func (e WebhookDeliveryRejected) Activity() ActivityEvent {
	d := triggerDetails(e.Trigger)
	d["reason"] = e.Reason
	return e.routine(e.Routine, RoutineWebhookRejectedAction, d)
}
