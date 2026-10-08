package infra

import (
	"context"
	"encoding/json"
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

func (r approvalRecord) toDomain(issueIDs []uint64) (domain.Approval, error) {
	var p boardApprovalPayload
	if err := json.Unmarshal([]byte(r.Payload), &p); err != nil {
		return domain.Approval{}, err
	}
	if p.Risks == nil {
		p.Risks = []string{}
	}
	if issueIDs == nil {
		issueIDs = []uint64{}
	}
	a := domain.Approval{
		ID: r.ID, GuildID: r.GuildID, Type: domain.ApprovalType(r.Type), Status: domain.ApprovalStatus(r.Status),
		Payload: domain.BoardApprovalPayload{
			Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
			NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
		},
		RequesterID: deref(r.RequestedByMemberID), DeciderID: deref(r.DecidedByMemberID),
		DecidedAt: utc(r.DecidedAt), IssueIDs: issueIDs,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.DecisionNote != nil {
		a.DecisionNote = *r.DecisionNote
	}
	return a, nil
}

func payloadJSON(p domain.BoardApprovalPayload) (string, error) {
	b, err := json.Marshal(boardApprovalPayload{
		Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
		NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
	})
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
		if err := tx.Raw(`INSERT INTO approvals (guild_id, type, status, payload, requested_by_member_id, created_at, updated_at)
			VALUES (?, ?, ?, ?::jsonb, ?, ?, ?) RETURNING id`,
			a.GuildID, string(a.Type), string(a.Status), payload, nullable(a.RequesterID), a.CreatedAt, a.UpdatedAt).Scan(&ids); err != nil {
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
	Body           string
	CreatedAt      time.Time
}

func (r approvalCommentRecord) toDomain() domain.ApprovalComment {
	return domain.ApprovalComment{ID: r.ID, ApprovalID: r.ApprovalID, AuthorID: deref(r.AuthorMemberID), Body: r.Body, CreatedAt: r.CreatedAt.UTC()}
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
	if err := s.query(ctx).Raw(`INSERT INTO approval_comments (approval_id, author_member_id, body, created_at) VALUES (?, ?, ?, ?) RETURNING *`,
		c.ApprovalID, nullable(c.AuthorID), c.Body, c.CreatedAt).Scan(&recs); err != nil {
		return domain.ApprovalComment{}, err
	}
	return recs[0].toDomain(), nil
}
