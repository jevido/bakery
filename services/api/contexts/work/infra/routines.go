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

type routineRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	GuildID           uint64
	ProjectID         *uint64
	GoalID            *uint64
	ParentIssueID     *uint64
	Title             string
	Description       string
	AssigneeAgentID   *uint64
	Priority          string
	Status            string
	ConcurrencyPolicy string
	CatchUpPolicy     string
	CreatedByMemberID *uint64
	CreatedByAgentID  *uint64
	LastTriggeredAt   *time.Time
	orm.Timestamps
}

func (routineRecord) TableName() string { return "routines" }

func (r routineRecord) toDomain() domain.Routine {
	out := domain.Routine{
		ID: r.ID, GuildID: r.GuildID, ProjectID: deref(r.ProjectID), GoalID: deref(r.GoalID), ParentIssueID: deref(r.ParentIssueID),
		AssigneeAgentID: deref(r.AssigneeAgentID), Title: r.Title, Description: r.Description,
		Priority: domain.Priority(r.Priority), Status: domain.RoutineStatus(r.Status),
		ConcurrencyPolicy: domain.ConcurrencyPolicy(r.ConcurrencyPolicy), CatchUpPolicy: domain.CatchUpPolicy(r.CatchUpPolicy),
		CreatedBy: actor(r.CreatedByMemberID, r.CreatedByAgentID),
	}
	if r.LastTriggeredAt != nil {
		t := r.LastTriggeredAt.UTC()
		out.LastTriggeredAt = &t
	}
	out.CreatedAt, out.UpdatedAt = stamp(&r.Timestamps)
	return out
}

// Routines keeps Routines.
type Routines struct{}

func (Routines) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

// Routines lists the Guild's Routines, newest first.
func (s Routines) Routines(ctx context.Context, guildID uint64) ([]domain.Routine, error) {
	var recs []routineRecord
	if err := s.query(ctx).Where("guild_id", guildID).Order("id desc").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Routine, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// Routine returns the Routine; found is false when there is none.
func (s Routines) Routine(ctx context.Context, id uint64) (domain.Routine, bool, error) {
	var rec routineRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Routine{}, false, nil
		}
		return domain.Routine{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Routines) CreateRoutine(ctx context.Context, r domain.Routine) (domain.Routine, error) {
	rec := routineRecord{
		GuildID: r.GuildID, ProjectID: nullable(r.ProjectID), GoalID: nullable(r.GoalID), ParentIssueID: nullable(r.ParentIssueID),
		Title: r.Title, Description: r.Description, AssigneeAgentID: nullable(r.AssigneeAgentID),
		Priority: string(r.Priority), Status: string(r.Status),
		ConcurrencyPolicy: string(r.ConcurrencyPolicy), CatchUpPolicy: string(r.CatchUpPolicy),
		CreatedByMemberID: nullable(r.CreatedBy.MemberID), CreatedByAgentID: nullable(r.CreatedBy.AgentID),
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Routine{}, err
	}
	return rec.toDomain(), nil
}

func (s Routines) SaveRoutine(ctx context.Context, r domain.Routine) error {
	_, err := s.query(ctx).Exec(`UPDATE routines SET project_id = ?, goal_id = ?, parent_issue_id = ?, title = ?, description = ?,
		assignee_agent_id = ?, priority = ?, status = ?, concurrency_policy = ?, catch_up_policy = ?, updated_at = now() WHERE id = ?`,
		nullable(r.ProjectID), nullable(r.GoalID), nullable(r.ParentIssueID), r.Title, r.Description,
		nullable(r.AssigneeAgentID), string(r.Priority), string(r.Status), string(r.ConcurrencyPolicy), string(r.CatchUpPolicy), r.ID)
	return err
}

// RoutinesOfAgent lists the Guild's Routines the Agent is the Agent
// assignee of, archived ones too.
func (s Routines) RoutinesOfAgent(ctx context.Context, guildID, agentID uint64) ([]domain.Routine, error) {
	var recs []routineRecord
	if err := s.query(ctx).Where("guild_id", guildID).Where("assignee_agent_id", agentID).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Routine, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// DeleteRoutinesOfProject deletes the Routines of a Project that was
// deleted.
func (s Routines) DeleteRoutinesOfProject(ctx context.Context, projectID uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM routines WHERE project_id = ?`, projectID)
	return err
}
