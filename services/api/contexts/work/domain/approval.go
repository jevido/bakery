package domain

import (
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
	ApprovalPending   ApprovalStatus = "pending"
	RevisionRequested ApprovalStatus = "revision_requested"
	ApprovalApproved  ApprovalStatus = "approved"
	ApprovalRejected  ApprovalStatus = "rejected"
)

// ApprovalStatuses lists every Approval status.
var ApprovalStatuses = []ApprovalStatus{ApprovalPending, RevisionRequested, ApprovalApproved, ApprovalRejected}

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
	return s == ApprovalPending || s == RevisionRequested
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
	return Approval{GuildID: guildID, Type: t, Status: ApprovalPending, Payload: p, RequesterID: requesterID, IssueIDs: ids}, nil
}

// Actionable is whether the Approval still waits for a Decision.
func (a Approval) Actionable() bool { return a.Status.Actionable() }
