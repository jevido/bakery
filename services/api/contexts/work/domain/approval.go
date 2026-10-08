package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ApprovalType is what an Approval asks the Board to decide. Only
// request_board_approval exists until agents come.
type ApprovalType string

const RequestBoardApproval ApprovalType = "request_board_approval"

// ApprovalTypes lists every Approval type.
var ApprovalTypes = []ApprovalType{RequestBoardApproval}

// ParseApprovalType reads an Approval type by its wire key.
func ParseApprovalType(s string) (ApprovalType, error) {
	if t := ApprovalType(s); slices.Contains(ApprovalTypes, t) {
		return t, nil
	}
	return "", invalid("type", "type must be request_board_approval")
}

// ApprovalStatus is where an Approval stands.
type ApprovalStatus string

const (
	StatusPending           ApprovalStatus = "pending"
	StatusRevisionRequested ApprovalStatus = "revision_requested"
	StatusApproved          ApprovalStatus = "approved"
	StatusRejected          ApprovalStatus = "rejected"
)

// ApprovalStatuses lists every Approval status.
var ApprovalStatuses = []ApprovalStatus{StatusPending, StatusRevisionRequested, StatusApproved, StatusRejected}

// ParseApprovalStatus reads an Approval status by its wire key.
func ParseApprovalStatus(s string) (ApprovalStatus, error) {
	if st := ApprovalStatus(s); slices.Contains(ApprovalStatuses, st) {
		return st, nil
	}
	return "", invalid("status", "status must be pending, revision_requested, approved or rejected")
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

// Approval is a decision a Member asks the Board of a Guild to make, with
// the Issues it is about. RequesterID and DeciderID are 0 for none (or
// once that Member's account is gone).
type Approval struct {
	ID           uint64
	GuildID      uint64
	Type         ApprovalType
	Status       ApprovalStatus
	Payload      BoardApprovalPayload
	RequesterID  uint64
	DeciderID    uint64
	DecisionNote string
	DecidedAt    *time.Time
	IssueIDs     []uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RequestApproval is a pending Approval of the Guild by the Member about
// the Issues, already known to be the Guild's; duplicate ids are dropped.
func RequestApproval(guildID, requesterID uint64, t ApprovalType, p BoardApprovalPayload, issueIDs []uint64) (Approval, error) {
	p, err := p.Validated()
	if err != nil {
		return Approval{}, err
	}
	ids := []uint64{}
	for _, id := range issueIDs {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return Approval{GuildID: guildID, Type: t, Status: StatusPending, Payload: p, RequesterID: requesterID, IssueIDs: ids}, nil
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

// Approve is the Board's yes, from pending or revision_requested.
func (a *Approval) Approve(by uint64, note string, at time.Time) (bool, error) {
	return a.resolve(StatusApproved, "approved", by, note, at)
}

// Reject is the Board's no, from pending or revision_requested.
func (a *Approval) Reject(by uint64, note string, at time.Time) (bool, error) {
	return a.resolve(StatusRejected, "rejected", by, note, at)
}

// RequestRevision sends a pending Approval back to its Requester.
func (a *Approval) RequestRevision(by uint64, note string, at time.Time) error {
	if a.Status != StatusPending {
		return &ApprovalRefusedError{"Only pending approvals can request revision"}
	}
	return a.decide(StatusRevisionRequested, by, note, at)
}

// Resubmit makes a revision_requested Approval pending again, with a new
// payload when there is one; only its Requester may. It clears the
// decider, the Decision note and the decision time.
func (a *Approval) Resubmit(by uint64, p *BoardApprovalPayload, at time.Time) error {
	if by == 0 || by != a.RequesterID {
		return ErrNotRequester
	}
	if a.Status != StatusRevisionRequested {
		return &ApprovalRefusedError{"Only revision requested approvals can be resubmitted"}
	}
	if p != nil {
		v, err := p.Validated()
		if err != nil {
			return err
		}
		a.Payload = v
	}
	a.Status, a.DeciderID, a.DecisionNote, a.DecidedAt, a.UpdatedAt = StatusPending, 0, "", nil, at
	return nil
}

// ApprovalComment is a Member's message on an Approval. It is never edited
// or deleted. AuthorID is 0 once the author's account is gone.
type ApprovalComment struct {
	ID         uint64
	ApprovalID uint64
	AuthorID   uint64
	Body       string
	CreatedAt  time.Time
}

// NewApprovalComment is a comment by a Member on an Approval.
func NewApprovalComment(approvalID, authorID uint64, body string) (ApprovalComment, error) {
	b, err := commentBody(body)
	if err != nil {
		return ApprovalComment{}, err
	}
	return ApprovalComment{ApprovalID: approvalID, AuthorID: authorID, Body: b}, nil
}
