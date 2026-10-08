package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type issueRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	GuildID           uint64
	Number            int
	Title             string
	Description       string
	Status            string
	Priority          string
	AssigneeMemberID  *uint64
	ProjectID         *uint64
	GoalID            *uint64
	ParentID          *uint64
	CreatedByMemberID *uint64
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CancelledAt       *time.Time
	orm.Timestamps
}

func (issueRecord) TableName() string { return "issues" }

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (r issueRecord) toDomain() domain.Issue {
	i := domain.Issue{
		ID: r.ID, GuildID: r.GuildID, Number: r.Number, Title: r.Title, Description: r.Description,
		Status: domain.IssueStatus(r.Status), Priority: domain.Priority(r.Priority),
		AssigneeID: deref(r.AssigneeMemberID), ProjectID: deref(r.ProjectID), GoalID: deref(r.GoalID),
		ParentID: deref(r.ParentID), CreatedByID: deref(r.CreatedByMemberID),
		StartedAt: utc(r.StartedAt), CompletedAt: utc(r.CompletedAt), CancelledAt: utc(r.CancelledAt),
	}
	i.CreatedAt, i.UpdatedAt = stamp(&r.Timestamps)
	return i
}

func issuesOf(recs []issueRecord) []domain.Issue {
	out := make([]domain.Issue, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out
}

// Issues keeps Issues.
type Issues struct{}

func (Issues) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

// CreateIssue counts the Guild's Issues up in issue_counters and stores the
// Issue under that number in the same transaction, so two at once never
// share one and a deleted Issue's number is never reused.
func (Issues) CreateIssue(ctx context.Context, i domain.Issue) (domain.Issue, error) {
	rec := issueRecord{
		GuildID: i.GuildID, Title: i.Title, Description: i.Description, Status: string(i.Status), Priority: string(i.Priority),
		AssigneeMemberID: nullable(i.AssigneeID), ProjectID: nullable(i.ProjectID), GoalID: nullable(i.GoalID),
		ParentID: nullable(i.ParentID), CreatedByMemberID: nullable(i.CreatedByID),
		StartedAt: i.StartedAt, CompletedAt: i.CompletedAt, CancelledAt: i.CancelledAt,
	}
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if err := tx.Raw(`INSERT INTO issue_counters (guild_id, last_number) VALUES (?, 1)
			ON CONFLICT (guild_id) DO UPDATE SET last_number = issue_counters.last_number + 1
			RETURNING last_number`, i.GuildID).Scan(&rec.Number); err != nil {
			return err
		}
		return tx.Create(&rec)
	})
	if err != nil {
		return domain.Issue{}, err
	}
	return rec.toDomain(), nil
}

func (s Issues) first(q contractsorm.Query) (domain.Issue, bool, error) {
	var rec issueRecord
	if err := q.FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Issue{}, false, nil
		}
		return domain.Issue{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Issues) Issue(ctx context.Context, id uint64) (domain.Issue, bool, error) {
	return s.first(s.query(ctx).Where("id", id))
}

func (s Issues) IssueByNumber(ctx context.Context, guildID uint64, number int) (domain.Issue, bool, error) {
	return s.first(s.query(ctx).Where("guild_id", guildID).Where("number", number))
}

func (s Issues) IssuesByID(ctx context.Context, ids []uint64) ([]domain.Issue, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var recs []issueRecord
	if err := s.query(ctx).Where("id IN ?", ids).Find(&recs); err != nil {
		return nil, err
	}
	return issuesOf(recs), nil
}

func (s Issues) SaveIssue(ctx context.Context, i domain.Issue) error {
	return s.save(s.query(ctx), i)
}

func (Issues) save(q contractsorm.Query, i domain.Issue) error {
	_, err := q.Exec(`UPDATE issues SET title = ?, description = ?, status = ?, priority = ?,
		assignee_member_id = ?, project_id = ?, goal_id = ?, parent_id = ?,
		started_at = ?, completed_at = ?, cancelled_at = ?, updated_at = now() WHERE id = ?`,
		i.Title, i.Description, string(i.Status), string(i.Priority),
		nullable(i.AssigneeID), nullable(i.ProjectID), nullable(i.GoalID), nullable(i.ParentID),
		i.StartedAt, i.CompletedAt, i.CancelledAt, i.ID)
	return err
}

// SaveIssueBlockedBy stores the Issue and its Blockers together: the rows
// no longer named go, the new ones are added, and the ones kept keep who
// added them.
func (s Issues) SaveIssueBlockedBy(ctx context.Context, i domain.Issue, blockedBy []uint64, memberID uint64) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if err := s.save(tx, i); err != nil {
			return err
		}
		if len(blockedBy) == 0 {
			_, err := tx.Exec(`DELETE FROM issue_blockers WHERE issue_id = ?`, i.ID)
			return err
		}
		if _, err := tx.Exec(`DELETE FROM issue_blockers WHERE issue_id = ? AND NOT (blocker_id IN ?)`, i.ID, blockedBy); err != nil {
			return err
		}
		for _, id := range blockedBy {
			if _, err := tx.Exec(`INSERT INTO issue_blockers (guild_id, issue_id, blocker_id, created_by_member_id)
				VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`, i.GuildID, i.ID, id, nullable(memberID)); err != nil {
				return err
			}
		}
		return nil
	})
}

// edges reads issue_blockers rows whose from column is one of ids, as the
// to column's ids per from id.
func (s Issues) edges(ctx context.Context, from, to string, ids []uint64) (map[uint64][]uint64, error) {
	out := map[uint64][]uint64{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		From uint64
		To   uint64
	}
	if err := s.query(ctx).Raw(`SELECT `+from+` AS "from", `+to+` AS "to" FROM issue_blockers WHERE `+from+` IN ?`, ids).Scan(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.From] = append(out[r.From], r.To)
	}
	return out, nil
}

func (s Issues) Blockers(ctx context.Context, issueIDs []uint64) (map[uint64][]uint64, error) {
	return s.edges(ctx, "issue_id", "blocker_id", issueIDs)
}

func (s Issues) Blocking(ctx context.Context, issueIDs []uint64) (map[uint64][]uint64, error) {
	return s.edges(ctx, "blocker_id", "issue_id", issueIDs)
}

// BlockedFrom walks the blocking edges out of the Issue; UNION (not UNION
// ALL) drops repeats, so a stored cycle cannot make it loop.
func (s Issues) BlockedFrom(ctx context.Context, issueID uint64) ([]uint64, error) {
	var ids []uint64
	err := s.query(ctx).Raw(`WITH RECURSIVE blocked(id) AS (
			SELECT issue_id FROM issue_blockers WHERE blocker_id = ?
			UNION
			SELECT b.issue_id FROM issue_blockers b JOIN blocked ON b.blocker_id = blocked.id
		) SELECT id FROM blocked`, issueID).Scan(&ids)
	return ids, err
}

// DeleteIssue deletes the Issue; the foreign key takes its Sub-issues'
// parent away.
func (s Issues) DeleteIssue(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM issues WHERE id = ?`, id)
	return err
}

// none matches a nullable column that is NULL when id is 0, else equal to
// id.
func none(q contractsorm.Query, column string, id *uint64) contractsorm.Query {
	switch {
	case id == nil:
		return q
	case *id == 0:
		return q.Where(column + " IS NULL")
	}
	return q.Where(column+" = ?", *id)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s Issues) FindIssues(ctx context.Context, guildID uint64, f app.IssueQuery) ([]domain.Issue, error) {
	q := s.query(ctx).Where("guild_id", guildID)
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	if len(f.Priorities) > 0 {
		q = q.Where("priority IN ?", f.Priorities)
	}
	q = none(q, "assignee_member_id", f.AssigneeID)
	q = none(q, "project_id", f.ProjectID)
	q = none(q, "goal_id", f.GoalID)
	q = none(q, "parent_id", f.ParentID)
	if len(f.Visible) > 0 {
		q = q.Where("(project_id IS NULL OR project_id IN ?)", f.Visible)
	} else {
		q = q.Where("project_id IS NULL")
	}
	if f.Search != "" {
		like := "%" + likeEscaper.Replace(f.Search) + "%"
		q = q.Where("(title ILIKE ? OR description ILIKE ? OR ? || '-' || number::text ILIKE ?)", like, like, f.Prefix, like)
	}
	q = q.Order("updated_at desc").Order("id desc")
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	var recs []issueRecord
	if err := q.Find(&recs); err != nil {
		return nil, err
	}
	return issuesOf(recs), nil
}

func (s Issues) IssueProjects(ctx context.Context, guildID uint64) ([]uint64, error) {
	var ids []uint64
	err := s.query(ctx).Raw(`SELECT DISTINCT project_id FROM issues WHERE guild_id = ? AND project_id IS NOT NULL ORDER BY project_id`, guildID).Scan(&ids)
	return ids, err
}

func (s Issues) LeaveProject(ctx context.Context, projectID uint64) error {
	_, err := s.query(ctx).Exec(`UPDATE issues SET project_id = NULL, updated_at = now() WHERE project_id = ?`, projectID)
	return err
}

func (s Issues) HasIssues(ctx context.Context, guildID uint64) (bool, error) {
	var n int64
	err := s.query(ctx).Raw(`SELECT count(*) FROM issues WHERE guild_id = ?`, guildID).Scan(&n)
	return n > 0, err
}
