package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ApprovalType is what an Approval asks the Board to decide.
type ApprovalType string

const (
	RequestBoardApproval ApprovalType = "request_board_approval"
	HireAgent            ApprovalType = "hire_agent"
	// BudgetOverrideRequired asks whether to raise a Budget whose Hard
	// stop was reached; only resolving its Budget incident decides it.
	BudgetOverrideRequired ApprovalType = "budget_override_required"
)

// ApprovalTypes lists every Approval type.
var ApprovalTypes = []ApprovalType{RequestBoardApproval, HireAgent, BudgetOverrideRequired}

// ParseApprovalType reads an Approval type by its wire key.
func ParseApprovalType(s string) (ApprovalType, error) {
	if t := ApprovalType(s); slices.Contains(ApprovalTypes, t) {
		return t, nil
	}
	keys := make([]string, len(ApprovalTypes))
	for i, t := range ApprovalTypes {
		keys[i] = string(t)
	}
	return "", invalid("type", "type must be %s", strings.Join(keys, " or "))
}

// ApprovalStatus is where an Approval stands.
type ApprovalStatus string

const (
	StatusPending           ApprovalStatus = "pending"
	StatusRevisionRequested ApprovalStatus = "revision_requested"
	StatusApproved          ApprovalStatus = "approved"
	StatusRejected          ApprovalStatus = "rejected"
	// StatusCancelled is a hire_agent Approval whose Agent was terminated
	// before a Decision, or a budget_override_required one whose Budget
	// went.
	StatusCancelled ApprovalStatus = "cancelled"
)

// ApprovalStatuses lists every Approval status.
var ApprovalStatuses = []ApprovalStatus{StatusPending, StatusRevisionRequested, StatusApproved, StatusRejected, StatusCancelled}

// ParseApprovalStatus reads an Approval status by its wire key.
func ParseApprovalStatus(s string) (ApprovalStatus, error) {
	if st := ApprovalStatus(s); slices.Contains(ApprovalStatuses, st) {
		return st, nil
	}
	return "", invalid("status", "status must be pending, revision_requested, approved, rejected or cancelled")
}

// Actionable is whether an Approval in this status still waits for a
// Decision.
func (s ApprovalStatus) Actionable() bool {
	return s == StatusPending || s == StatusRevisionRequested
}

// The limits of a request_board_approval payload, in characters.
const (
	MaxApprovalText = 20000
	MaxRisks        = 20
	MaxRisk         = 500
)

// ApprovalPayload is what an Approval asks, one kind per Approval type:
// BoardApprovalPayload or HireAgentPayload.
type ApprovalPayload interface {
	// Type is the Approval type this payload belongs to.
	Type() ApprovalType
	// Label is the Approval's title, as Paperclip's approvalLabel.
	Label() string
	validate() (ApprovalPayload, error)
}

// BoardApprovalPayload is what a request_board_approval asks: a title,
// and optionally a summary, a recommended action, what happens on approval
// and its risks.
type BoardApprovalPayload struct {
	Title                string
	Summary              string
	RecommendedAction    string
	NextActionOnApproval string
	Risks                []string
}

func (BoardApprovalPayload) Type() ApprovalType { return RequestBoardApproval }

func (p BoardApprovalPayload) Label() string { return p.Title }

func (p BoardApprovalPayload) validate() (ApprovalPayload, error) { return p.Validated() }

// Validated is the payload trimmed and checked against the work
// document's rules; empty risks are dropped.
func (p BoardApprovalPayload) Validated() (BoardApprovalPayload, error) {
	t, err := Title("payload.title", p.Title)
	if err != nil {
		return BoardApprovalPayload{}, err
	}
	out := BoardApprovalPayload{Title: t, Risks: []string{}}
	for _, f := range []struct {
		field string
		in    string
		into  *string
	}{
		{"payload.summary", p.Summary, &out.Summary},
		{"payload.recommended_action", p.RecommendedAction, &out.RecommendedAction},
		{"payload.next_action_on_approval", p.NextActionOnApproval, &out.NextActionOnApproval},
	} {
		v := strings.TrimSpace(f.in)
		if utf8.RuneCountInString(v) > MaxApprovalText {
			return BoardApprovalPayload{}, invalid(f.field, "at most %d characters", MaxApprovalText)
		}
		*f.into = v
	}
	for _, r := range p.Risks {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if utf8.RuneCountInString(r) > MaxRisk {
			return BoardApprovalPayload{}, invalid("payload.risks", "each risk is at most %d characters", MaxRisk)
		}
		out.Risks = append(out.Risks, r)
	}
	if len(out.Risks) > MaxRisks {
		return BoardApprovalPayload{}, invalid("payload.risks", "at most %d risks", MaxRisks)
	}
	return out, nil
}

// The limits of a hire_agent payload, in characters, as the agents
// document caps an Agent.
const (
	MaxAgentName         = 100
	MaxAgentJob          = 50
	MaxAgentTitle        = 200
	MaxAgentIcon         = 50
	MaxAgentCapabilities = 20000
	MaxHireRoles         = 50
	MaxHireRoleName      = 100
)

// HireAgentPayload is what a hire_agent asks: the Agent waiting to be
// hired, as the agents context describes it. ManagerID is 0 when it
// reports to no one.
type HireAgentPayload struct {
	AgentID      uint64
	Name         string
	Job          string
	Title        string
	Icon         string
	Capabilities string
	ManagerID    uint64
	ManagerName  string
	Roles        []string
}

func (HireAgentPayload) Type() ApprovalType { return HireAgent }

func (p HireAgentPayload) Label() string { return "Hire Agent: " + p.Name }

func (p HireAgentPayload) validate() (ApprovalPayload, error) { return p.Validated() }

// Validated is the payload trimmed and checked: an Agent, a name, and
// nothing longer than an Agent may hold.
func (p HireAgentPayload) Validated() (HireAgentPayload, error) {
	if p.AgentID == 0 {
		return HireAgentPayload{}, invalid("payload.agent_id", "agent_id is required")
	}
	out := HireAgentPayload{AgentID: p.AgentID, ManagerID: p.ManagerID, Roles: []string{}}
	for _, f := range []struct {
		field string
		in    string
		max   int
		into  *string
	}{
		{"payload.name", p.Name, MaxAgentName, &out.Name},
		{"payload.job", p.Job, MaxAgentJob, &out.Job},
		{"payload.title", p.Title, MaxAgentTitle, &out.Title},
		{"payload.icon", p.Icon, MaxAgentIcon, &out.Icon},
		{"payload.capabilities", p.Capabilities, MaxAgentCapabilities, &out.Capabilities},
		{"payload.reports_to", p.ManagerName, MaxAgentName, &out.ManagerName},
	} {
		v := strings.TrimSpace(f.in)
		if utf8.RuneCountInString(v) > f.max {
			return HireAgentPayload{}, invalid(f.field, "at most %d characters", f.max)
		}
		*f.into = v
	}
	if out.Name == "" {
		return HireAgentPayload{}, invalid("payload.name", "name is required")
	}
	if out.ManagerID == 0 {
		out.ManagerName = ""
	}
	if len(p.Roles) > MaxHireRoles {
		return HireAgentPayload{}, invalid("payload.roles", "at most %d roles", MaxHireRoles)
	}
	for _, r := range p.Roles {
		r = strings.TrimSpace(r)
		if r == "" || utf8.RuneCountInString(r) > MaxHireRoleName {
			return HireAgentPayload{}, invalid("payload.roles", "each role is 1 to %d characters", MaxHireRoleName)
		}
		out.Roles = append(out.Roles, r)
	}
	return out, nil
}

// BudgetOverrideGuidance is what a budget_override_required Approval
// tells the Board to do.
const BudgetOverrideGuidance = "Raise the budget and resume the scope, or keep the scope paused."

// BudgetOverridePayload is what a budget_override_required asks: the
// Budget whose Hard stop was reached, as the agents context describes it.
// The window bounds are nil for a lifetime Budget.
type BudgetOverridePayload struct {
	BudgetID    uint64
	ScopeType   string
	ScopeID     uint64
	ScopeName   string
	Metric      string
	Window      string
	Threshold   string
	Amount      int64
	Observed    int64
	WarnPercent int
	WindowStart *time.Time
	WindowEnd   *time.Time
	Guidance    string
}

func (BudgetOverridePayload) Type() ApprovalType { return BudgetOverrideRequired }

func (p BudgetOverridePayload) Label() string { return "Budget override: " + p.ScopeName }

func (p BudgetOverridePayload) validate() (ApprovalPayload, error) {
	if p.BudgetID == 0 || p.ScopeID == 0 {
		return nil, invalid("payload.budget_id", "budget_id and scope_id are required")
	}
	p.ScopeName = strings.TrimSpace(p.ScopeName)
	if p.ScopeName == "" {
		return nil, invalid("payload.scope_name", "scope_name is required")
	}
	if p.Guidance == "" {
		p.Guidance = BudgetOverrideGuidance
	}
	return p, nil
}

// errResolveOnCostsPage refuses a generic Decision, Request revision or
// Resubmit on a budget_override_required Approval.
var errResolveOnCostsPage = &ApprovalRefusedError{"resolve the budget incident on the Costs page"}

// Approval is a decision a Member or an Agent asks the Board of a Guild to
// make, with the Issues it is about. The Requester is nobody and DeciderID
// 0 for none (or once that account or Agent is gone); only a Member decides.
type Approval struct {
	ID           uint64
	GuildID      uint64
	Type         ApprovalType
	Status       ApprovalStatus
	Payload      ApprovalPayload
	Requester    Actor
	DeciderID    uint64
	DecisionNote string
	DecidedAt    *time.Time
	IssueIDs     []uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RequestApproval is a pending Approval of the Guild by the Member or Agent about
// the Issues, already known to be the Guild's; duplicate ids are dropped.
// The payload must be the type's kind.
func RequestApproval(guildID uint64, requester Actor, t ApprovalType, p ApprovalPayload, issueIDs []uint64) (Approval, error) {
	if p == nil || p.Type() != t {
		return Approval{}, invalid("payload", "payload does not fit type %s", t)
	}
	p, err := p.validate()
	if err != nil {
		return Approval{}, err
	}
	ids := []uint64{}
	for _, id := range issueIDs {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return Approval{GuildID: guildID, Type: t, Status: StatusPending, Payload: p, Requester: requester, IssueIDs: ids}, nil
}

// Actionable is whether the Approval still waits for a Decision.
func (a Approval) Actionable() bool { return a.Status.Actionable() }

// ApprovalRefusedError is a move the Approval's status does not allow;
// the HTTP layer answers it 422.
type ApprovalRefusedError struct{ Message string }

func (e *ApprovalRefusedError) Error() string { return e.Message }

// ErrNotRequester is a Resubmit by anyone but the Approval's Requester.
var ErrNotRequester = errors.New("Only the Requester can resubmit this approval")

// MaxDecisionNote is the longest Decision note, in characters.
const MaxDecisionNote = 20000

func decisionNote(note string) (string, error) {
	n := strings.TrimSpace(note)
	if utf8.RuneCountInString(n) > MaxDecisionNote {
		return "", invalid("decision_note", "decision_note is at most %d characters", MaxDecisionNote)
	}
	return n, nil
}

// decide makes a Decision to the status, recording its decider, time and
// note.
func (a *Approval) decide(to ApprovalStatus, by uint64, note string, at time.Time) error {
	n, err := decisionNote(note)
	if err != nil {
		return err
	}
	a.Status, a.DeciderID, a.DecisionNote, a.DecidedAt, a.UpdatedAt = to, by, n, &at, at
	return nil
}

// resolve approves or rejects an Actionable Approval. One that already
// has that status is unchanged (changed is false), as Paperclip's
// applied: false.
func (a *Approval) resolve(to ApprovalStatus, verb string, by uint64, note string, at time.Time) (bool, error) {
	if !a.Actionable() {
		if a.Status == to {
			return false, nil
		}
		return false, &ApprovalRefusedError{"Only pending or revision requested approvals can be " + verb}
	}
	return true, a.decide(to, by, note, at)
}

// Approve is the Board's yes, from pending or revision_requested; never
// on a budget_override_required Approval (DecideBudgetOverride).
func (a *Approval) Approve(by uint64, note string, at time.Time) (bool, error) {
	if a.Type == BudgetOverrideRequired {
		return false, errResolveOnCostsPage
	}
	return a.resolve(StatusApproved, "approved", by, note, at)
}

// Reject is the Board's no, from pending or revision_requested; never on
// a budget_override_required Approval (DecideBudgetOverride).
func (a *Approval) Reject(by uint64, note string, at time.Time) (bool, error) {
	if a.Type == BudgetOverrideRequired {
		return false, errResolveOnCostsPage
	}
	return a.resolve(StatusRejected, "rejected", by, note, at)
}

// DecideBudgetOverride approves or rejects a budget_override_required
// Approval, as its Budget incident was resolved by the Member.
func (a *Approval) DecideBudgetOverride(approved bool, by uint64, note string, at time.Time) (bool, error) {
	if a.Type != BudgetOverrideRequired {
		return false, &ApprovalRefusedError{"Only budget override approvals are decided by resolving a budget incident"}
	}
	if approved {
		return a.resolve(StatusApproved, "approved", by, note, at)
	}
	return a.resolve(StatusRejected, "rejected", by, note, at)
}

// Cancel ends an Actionable hire_agent Approval whose Agent was
// terminated before a Decision, or a budget_override_required one whose
// Budget went with its Agent or Project. One already cancelled is unchanged
// (changed is false).
func (a *Approval) Cancel(at time.Time) (bool, error) {
	if a.Status == StatusCancelled {
		return false, nil
	}
	if a.Type != HireAgent && a.Type != BudgetOverrideRequired {
		return false, &ApprovalRefusedError{"Only hire agent and budget override approvals can be cancelled"}
	}
	if !a.Actionable() {
		return false, &ApprovalRefusedError{"Only pending or revision requested approvals can be cancelled"}
	}
	a.Status, a.UpdatedAt = StatusCancelled, at
	return true, nil
}

// RequestRevision sends a pending Approval back to its Requester. A hire
// never goes back: its Agent cannot change while it waits.
func (a *Approval) RequestRevision(by uint64, note string, at time.Time) error {
	if a.Type == BudgetOverrideRequired {
		return errResolveOnCostsPage
	}
	if a.Type == HireAgent {
		return &ApprovalRefusedError{"Hire agent approvals cannot be sent back for revision"}
	}
	if a.Status != StatusPending {
		return &ApprovalRefusedError{"Only pending approvals can request revision"}
	}
	return a.decide(StatusRevisionRequested, by, note, at)
}

// Resubmit makes a revision_requested Approval pending again, with a new
// payload when there is one; only its Requester may. It clears the
// decider, the Decision note and the decision time. A hire's payload
// never changes.
func (a *Approval) Resubmit(by Actor, p ApprovalPayload, at time.Time) error {
	if a.Type == BudgetOverrideRequired {
		return errResolveOnCostsPage
	}
	if !by.Is(a.Requester) {
		return ErrNotRequester
	}
	if a.Status != StatusRevisionRequested {
		return &ApprovalRefusedError{"Only revision requested approvals can be resubmitted"}
	}
	if p != nil {
		if a.Type == HireAgent {
			return &ApprovalRefusedError{"A hire agent approval's payload cannot change"}
		}
		if p.Type() != a.Type {
			return invalid("payload", "payload does not fit type %s", a.Type)
		}
		v, err := p.validate()
		if err != nil {
			return err
		}
		a.Payload = v
	}
	a.Status, a.DeciderID, a.DecisionNote, a.DecidedAt, a.UpdatedAt = StatusPending, 0, "", nil, at
	return nil
}

// ApprovalComment is a Member's or an Agent's message on an Approval. It
// is never edited or deleted. Its Author is nobody once that account or
// Agent is gone.
type ApprovalComment struct {
	ID         uint64
	ApprovalID uint64
	Author     Actor
	Body       string
	CreatedAt  time.Time
}

// NewApprovalComment is a comment by a Member or an Agent on an Approval.
func NewApprovalComment(approvalID uint64, author Actor, body string) (ApprovalComment, error) {
	b, err := commentBody(body)
	if err != nil {
		return ApprovalComment{}, err
	}
	return ApprovalComment{ApprovalID: approvalID, Author: author, Body: b}, nil
}
