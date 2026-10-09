package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type approvalRecord struct {
	ID                  uint64
	GuildID             uint64
	Type                string
	Status              string
	Payload             string
	RequestedByMemberID *uint64
	RequestedByAgentID  *uint64
	DecidedByMemberID   *uint64
	DecisionNote        *string
	DecidedAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// boardApprovalPayload is a request_board_approval payload as stored, in
// the snake_case it has on the wire.
type boardApprovalPayload struct {
	Title                string   `json:"title"`
	Summary              string   `json:"summary"`
	RecommendedAction    string   `json:"recommended_action"`
	NextActionOnApproval string   `json:"next_action_on_approval"`
	Risks                []string `json:"risks"`
}

// agentRef is an Agent as a hire_agent payload names it.
type agentRef struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// hireAgentPayload is a hire_agent payload as stored, in the snake_case it
// has on the wire.
type hireAgentPayload struct {
	AgentID      uint64    `json:"agent_id"`
	Name         string    `json:"name"`
	Job          string    `json:"job"`
	Title        string    `json:"title"`
	Icon         string    `json:"icon"`
	Capabilities string    `json:"capabilities"`
	ReportsTo    *agentRef `json:"reports_to"`
	Roles        []string  `json:"roles"`
}

// budgetOverridePayload is a budget_override_required payload as stored,
// in the snake_case it has on the wire.
type budgetOverridePayload struct {
	BudgetID    uint64     `json:"budget_id"`
	ScopeType   string     `json:"scope_type"`
	ScopeID     uint64     `json:"scope_id"`
	ScopeName   string     `json:"scope_name"`
	Metric      string     `json:"metric"`
	Window      string     `json:"window"`
	Threshold   string     `json:"threshold"`
	Amount      int64      `json:"amount"`
	Observed    int64      `json:"observed"`
	WarnPercent int        `json:"warn_percent"`
	WindowStart *time.Time `json:"window_start"`
	WindowEnd   *time.Time `json:"window_end"`
	Guidance    string     `json:"guidance"`
}

// payloadOf reads a stored payload by the Approval's type.
func payloadOf(typ domain.ApprovalType, raw string) (domain.ApprovalPayload, error) {
	if typ == domain.BudgetOverrideRequired {
		var p budgetOverridePayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		return domain.BudgetOverridePayload(p), nil
	}
	if typ == domain.HireAgent {
		var p hireAgentPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		out := domain.HireAgentPayload{AgentID: p.AgentID, Name: p.Name, Job: p.Job, Title: p.Title, Icon: p.Icon, Capabilities: p.Capabilities, Roles: p.Roles}
		if p.ReportsTo != nil {
			out.ManagerID, out.ManagerName = p.ReportsTo.ID, p.ReportsTo.Name
		}
		if out.Roles == nil {
			out.Roles = []string{}
		}
		return out, nil
	}
	var p boardApprovalPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	if p.Risks == nil {
		p.Risks = []string{}
	}
	return domain.BoardApprovalPayload{
		Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
		NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
	}, nil
}

func (r approvalRecord) toDomain(issueIDs []uint64) (domain.Approval, error) {
	p, err := payloadOf(domain.ApprovalType(r.Type), r.Payload)
	if err != nil {
		return domain.Approval{}, err
	}
	if issueIDs == nil {
		issueIDs = []uint64{}
	}
	a := domain.Approval{
		ID: r.ID, GuildID: r.GuildID, Type: domain.ApprovalType(r.Type), Status: domain.ApprovalStatus(r.Status),
		Payload:   p,
		Requester: actor(r.RequestedByMemberID, r.RequestedByAgentID), DeciderID: deref(r.DecidedByMemberID),
		DecidedAt: utc(r.DecidedAt), IssueIDs: issueIDs,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.DecisionNote != nil {
		a.DecisionNote = *r.DecisionNote
	}
	return a, nil
}

func payloadJSON(p domain.ApprovalPayload) (string, error) {
	var v any
	switch p := p.(type) {
	case domain.HireAgentPayload:
		h := hireAgentPayload{AgentID: p.AgentID, Name: p.Name, Job: p.Job, Title: p.Title, Icon: p.Icon, Capabilities: p.Capabilities, Roles: p.Roles}
		if p.ManagerID != 0 {
			h.ReportsTo = &agentRef{ID: p.ManagerID, Name: p.ManagerName}
		}
		v = h
	case domain.BudgetOverridePayload:
		v = budgetOverridePayload(p)
	case domain.BoardApprovalPayload:
		v = boardApprovalPayload{
			Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
			NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
		}
	default:
		return "", fmt.Errorf("approval payload %T cannot be stored", p)
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// Approvals keeps Approvals and their Linked issues.
type Approvals struct{}

func (Approvals) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

// CreateApproval inserts the Approval and its Linked issues in one
// transaction.
func (s Approvals) CreateApproval(ctx context.Context, a domain.Approval) (domain.Approval, error) {
	payload, err := payloadJSON(a.Payload)
	if err != nil {
		return domain.Approval{}, err
	}
	var id uint64
	err = facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		var ids []uint64
		if err := tx.Raw(`INSERT INTO approvals (guild_id, type, status, payload, requested_by_member_id, requested_by_agent_id, created_at, updated_at)
			VALUES (?, ?, ?, ?::jsonb, ?, ?, ?, ?) RETURNING id`,
			a.GuildID, string(a.Type), string(a.Status), payload, nullable(a.Requester.MemberID), nullable(a.Requester.AgentID), a.CreatedAt, a.UpdatedAt).Scan(&ids); err != nil {
			return err
		}
		id = ids[0]
		for _, issueID := range a.IssueIDs {
			if _, err := tx.Exec(`INSERT INTO approval_issues (approval_id, issue_id, created_at) VALUES (?, ?, ?)`, id, issueID, a.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Approval{}, err
	}
	saved, _, err := s.Approval(ctx, id)
	return saved, err
}

// Approval returns the Approval; found is false when there is none.
func (s Approvals) Approval(ctx context.Context, id uint64) (domain.Approval, bool, error) {
	as, err := s.find(ctx, `WHERE id = ?`, id)
	if err != nil || len(as) == 0 {
		return domain.Approval{}, false, err
	}
	return as[0], true, nil
}

// Approvals lists the Guild's Approvals in these statuses (any for none),
// newest first.
func (s Approvals) Approvals(ctx context.Context, guildID uint64, statuses []domain.ApprovalStatus) ([]domain.Approval, error) {
	if len(statuses) == 0 {
		return s.find(ctx, `WHERE guild_id = ? ORDER BY id DESC`, guildID)
	}
	keys := make([]string, len(statuses))
	for i, st := range statuses {
		keys[i] = string(st)
	}
	return s.find(ctx, `WHERE guild_id = ? AND status IN ? ORDER BY id DESC`, guildID, keys)
}

// CountApprovals counts the Guild's Approvals in these statuses (any for
// none).
func (s Approvals) CountApprovals(ctx context.Context, guildID uint64, statuses []domain.ApprovalStatus) (int64, error) {
	q := s.query(ctx).Table("approvals").Where("guild_id = ?", guildID)
	if len(statuses) > 0 {
		keys := make([]string, len(statuses))
		for i, st := range statuses {
			keys[i] = string(st)
		}
		q = q.Where("status IN ?", keys)
	}
	return q.Count()
}

// IssueApprovals lists the Approvals linked to the Issue, newest first.
func (s Approvals) IssueApprovals(ctx context.Context, issueID uint64) ([]domain.Approval, error) {
	return s.find(ctx, `WHERE id IN (SELECT approval_id FROM approval_issues WHERE issue_id = ?) ORDER BY id DESC`, issueID)
}

// find reads the Approvals the clause picks, with their Linked issues
// asked for in one go.
func (s Approvals) find(ctx context.Context, clause string, args ...any) ([]domain.Approval, error) {
	var recs []approvalRecord
	if err := s.query(ctx).Raw(`SELECT * FROM approvals `+clause, args...).Scan(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Approval, 0, len(recs))
	if len(recs) == 0 {
		return out, nil
	}
	ids := make([]uint64, len(recs))
	for i, r := range recs {
		ids[i] = r.ID
	}
	var links []struct {
		ApprovalID uint64
		IssueID    uint64
	}
	if err := s.query(ctx).Raw(`SELECT approval_id, issue_id FROM approval_issues WHERE approval_id IN ? ORDER BY created_at, issue_id`, ids).Scan(&links); err != nil {
		return nil, err
	}
	issues := map[uint64][]uint64{}
	for _, l := range links {
		issues[l.ApprovalID] = append(issues[l.ApprovalID], l.IssueID)
	}
	for _, r := range recs {
		a, err := r.toDomain(issues[r.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// SaveApproval writes the Approval's status, payload and Decision only
// while it is still in one of the statuses it was moved from; saved is
// false when someone else moved it first.
func (s Approvals) SaveApproval(ctx context.Context, a domain.Approval, from []domain.ApprovalStatus) (bool, error) {
	payload, err := payloadJSON(a.Payload)
	if err != nil {
		return false, err
	}
	keys := make([]string, len(from))
	for i, st := range from {
		keys[i] = string(st)
	}
	var note *string
	if a.DecisionNote != "" {
		note = &a.DecisionNote
	}
	res, err := s.query(ctx).Exec(`UPDATE approvals SET status = ?, payload = ?::jsonb, decided_by_member_id = ?, decision_note = ?, decided_at = ?, updated_at = ?
		WHERE id = ? AND status IN ?`,
		string(a.Status), payload, nullable(a.DeciderID), note, a.DecidedAt, a.UpdatedAt, a.ID, keys)
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
}

type approvalCommentRecord struct {
	ID             uint64
	ApprovalID     uint64
	AuthorMemberID *uint64
	AuthorAgentID  *uint64
	Body           string
	CreatedAt      time.Time
}

func (r approvalCommentRecord) toDomain() domain.ApprovalComment {
	return domain.ApprovalComment{ID: r.ID, ApprovalID: r.ApprovalID, Author: actor(r.AuthorMemberID, r.AuthorAgentID), Body: r.Body, CreatedAt: r.CreatedAt.UTC()}
}

// ApprovalComments lists the Approval's comments, oldest first.
func (s Approvals) ApprovalComments(ctx context.Context, approvalID uint64) ([]domain.ApprovalComment, error) {
	var recs []approvalCommentRecord
	if err := s.query(ctx).Raw(`SELECT * FROM approval_comments WHERE approval_id = ? ORDER BY created_at, id`, approvalID).Scan(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.ApprovalComment, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (s Approvals) CreateApprovalComment(ctx context.Context, c domain.ApprovalComment) (domain.ApprovalComment, error) {
	var recs []approvalCommentRecord
	if err := s.query(ctx).Raw(`INSERT INTO approval_comments (approval_id, author_member_id, author_agent_id, body, created_at) VALUES (?, ?, ?, ?, ?) RETURNING *`,
		c.ApprovalID, nullable(c.Author.MemberID), nullable(c.Author.AgentID), c.Body, c.CreatedAt).Scan(&recs); err != nil {
		return domain.ApprovalComment{}, err
	}
	return recs[0].toDomain(), nil
}
