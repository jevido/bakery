package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type commentRecord struct {
	ID             uint64 `gorm:"primaryKey"`
	IssueID        uint64
	AuthorMemberID *uint64
	AuthorAgentID  *uint64
	RunID          *uint64
	Body           string
	DeletedAt      *time.Time
	orm.Timestamps
}

func (commentRecord) TableName() string { return "issue_comments" }

func (r commentRecord) toDomain() domain.Comment {
	c := domain.Comment{ID: r.ID, IssueID: r.IssueID, Author: actor(r.AuthorMemberID, r.AuthorAgentID), RunID: deref(r.RunID), Body: r.Body, DeletedAt: utc(r.DeletedAt)}
	c.CreatedAt, c.UpdatedAt = stamp(&r.Timestamps)
	return c
}

// Comments keeps the Comments on Issues.
type Comments struct{}

func (Comments) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Comments) Comments(ctx context.Context, issueID uint64) ([]domain.Comment, error) {
	var recs []commentRecord
	if err := s.query(ctx).Where("issue_id", issueID).Order("created_at").Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Comment, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (s Comments) Comment(ctx context.Context, id uint64) (domain.Comment, bool, error) {
	var rec commentRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Comment{}, false, nil
		}
		return domain.Comment{}, false, err
	}
	return rec.toDomain(), true, nil
}

// CreateComment stores the Comment and touches its Issue's updated_at in
// the same transaction. The Issue's rules do not depend on updated_at, so
// this does not change the Issue as an aggregate.
func (s Comments) CreateComment(ctx context.Context, c domain.Comment) (domain.Comment, error) {
	return s.create(ctx, c, nil)
}

// CreateConversationComment stores the Comment and moves its Conversation
// in the same transaction: the new state, and the Comment as Session
// boundary for a New session.
func (s Comments) CreateConversationComment(ctx context.Context, c domain.Comment, move domain.ConversationMove) (domain.Comment, error) {
	return s.create(ctx, c, &move)
}

func (Comments) create(ctx context.Context, c domain.Comment, move *domain.ConversationMove) (domain.Comment, error) {
	rec := commentRecord{IssueID: c.IssueID, AuthorMemberID: nullable(c.Author.MemberID), AuthorAgentID: nullable(c.Author.AgentID), RunID: nullable(c.RunID), Body: c.Body}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if err := tx.Create(&rec); err != nil {
			return err
		}
		if move == nil {
			_, err := tx.Exec(`UPDATE issues SET updated_at = now() WHERE id = ?`, c.IssueID)
			return err
		}
		_, err := tx.Exec(`UPDATE issues SET conversation_state = ?,
			conversation_boundary_comment_id = CASE WHEN ? THEN ?::bigint ELSE conversation_boundary_comment_id END,
			updated_at = now() WHERE id = ?`, string(move.State), move.NewSession, rec.ID, c.IssueID)
		return err
	})
	if err != nil {
		return domain.Comment{}, err
	}
	return rec.toDomain(), nil
}

func (s Comments) SaveComment(ctx context.Context, c domain.Comment) (domain.Comment, error) {
	if _, err := s.query(ctx).Exec(`UPDATE issue_comments SET body = ?, deleted_at = ?, updated_at = now() WHERE id = ?`,
		c.Body, c.DeletedAt, c.ID); err != nil {
		return domain.Comment{}, err
	}
	saved, _, err := s.Comment(ctx, c.ID)
	return saved, err
}
