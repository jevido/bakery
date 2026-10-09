package infra

import (
	"context"
	"database/sql/driver"
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

type routineTriggerRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	GuildID           uint64
	RoutineID         uint64
	Kind              string
	Label             string
	Enabled           bool
	CronExpression    string
	Timezone          string
	NextRunAt         *time.Time
	LastFiredAt       *time.Time
	LastResult        string
	CreatedByMemberID *uint64
	CreatedByAgentID  *uint64
	orm.Timestamps
}

func (routineTriggerRecord) TableName() string { return "routine_triggers" }

func (r routineTriggerRecord) toDomain() domain.RoutineTrigger {
	out := domain.RoutineTrigger{
		ID: r.ID, GuildID: r.GuildID, RoutineID: r.RoutineID, Kind: domain.TriggerKind(r.Kind), Label: r.Label, Enabled: r.Enabled,
		CronExpression: r.CronExpression, Timezone: r.Timezone, NextRunAt: utc(r.NextRunAt), LastFiredAt: utc(r.LastFiredAt),
		LastResult: r.LastResult, CreatedBy: actor(r.CreatedByMemberID, r.CreatedByAgentID),
	}
	out.CreatedAt, out.UpdatedAt = stamp(&r.Timestamps)
	return out
}

// Triggers lists the Routine triggers of the Routines, in the order they
// were added.
func (s Routines) Triggers(ctx context.Context, routineIDs []uint64) ([]domain.RoutineTrigger, error) {
	if len(routineIDs) == 0 {
		return nil, nil
	}
	var recs []routineTriggerRecord
	if err := s.query(ctx).Where("routine_id IN ?", routineIDs).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.RoutineTrigger, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// Trigger returns the Routine trigger; found is false when there is none.
func (s Routines) Trigger(ctx context.Context, id uint64) (domain.RoutineTrigger, bool, error) {
	var rec routineTriggerRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.RoutineTrigger{}, false, nil
		}
		return domain.RoutineTrigger{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Routines) CreateTrigger(ctx context.Context, t domain.RoutineTrigger) (domain.RoutineTrigger, error) {
	rec := routineTriggerRecord{
		GuildID: t.GuildID, RoutineID: t.RoutineID, Kind: string(t.Kind), Label: t.Label, Enabled: t.Enabled,
		CronExpression: t.CronExpression, Timezone: t.Timezone, NextRunAt: t.NextRunAt,
		CreatedByMemberID: nullable(t.CreatedBy.MemberID), CreatedByAgentID: nullable(t.CreatedBy.AgentID),
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.RoutineTrigger{}, err
	}
	return rec.toDomain(), nil
}

func (s Routines) SaveTrigger(ctx context.Context, t domain.RoutineTrigger) error {
	_, err := s.query(ctx).Exec(`UPDATE routine_triggers SET label = ?, enabled = ?, cron_expression = ?, timezone = ?, next_run_at = ?,
		last_fired_at = ?, last_result = ?, updated_at = now() WHERE id = ?`,
		t.Label, t.Enabled, t.CronExpression, t.Timezone, t.NextRunAt, t.LastFiredAt, t.LastResult, t.ID)
	return err
}

func (s Routines) DeleteTrigger(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM routine_triggers WHERE id = ?`, id)
	return err
}

// routineLocks is the first key of the advisory locks on Routines, so they
// never meet another use of pg_advisory_lock.
const routineLocks = 45

// LockRoutine takes a session advisory lock on the Routine on a
// connection of its own, held until unlock, so two API processes
// dispatching a Routine run of the same Routine take turns.
func (s Routines) LockRoutine(ctx context.Context, id uint64) (func(), error) {
	db, err := facades.Orm().DB()
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1, $2::int)`, routineLocks, int32(id)); err != nil {
		conn.Close()
		return nil, err
	}
	return func() {
		// The request may be gone already; the lock is still released.
		if _, err := conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1, $2::int)`, routineLocks, int32(id)); err != nil {
			// A connection that cannot unlock is closed: that ends its
			// session, and its lock with it.
			conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
	}, nil
}

// RoutineTriggered sets when the Routine last ran.
func (s Routines) RoutineTriggered(ctx context.Context, id uint64, at time.Time) error {
	_, err := s.query(ctx).Exec(`UPDATE routines SET last_triggered_at = ? WHERE id = ?`, at, id)
	return err
}

type routineRunRecord struct {
	ID                        uint64 `gorm:"primaryKey"`
	GuildID                   uint64
	RoutineID                 uint64
	TriggerID                 *uint64
	Source                    string
	Status                    string
	TriggeredAt               time.Time
	LinkedIssueID             *uint64
	CoalescedIntoRoutineRunID *uint64
	FailureReason             string
	TriggeredByMemberID       *uint64
	TriggeredByAgentID        *uint64
	CompletedAt               *time.Time
	orm.Timestamps
}

func (routineRunRecord) TableName() string { return "routine_runs" }

func (r routineRunRecord) toDomain() domain.RoutineRun {
	out := domain.RoutineRun{
		ID: r.ID, GuildID: r.GuildID, RoutineID: r.RoutineID, TriggerID: deref(r.TriggerID),
		Source: domain.RoutineRunSource(r.Source), Status: domain.RoutineRunStatus(r.Status), TriggeredAt: r.TriggeredAt.UTC(),
		LinkedIssueID: deref(r.LinkedIssueID), CoalescedIntoRunID: deref(r.CoalescedIntoRoutineRunID), FailureReason: r.FailureReason,
		TriggeredBy: actor(r.TriggeredByMemberID, r.TriggeredByAgentID), CompletedAt: utc(r.CompletedAt),
	}
	out.CreatedAt, out.UpdatedAt = stamp(&r.Timestamps)
	return out
}

func routineRunsOf(recs []routineRunRecord) []domain.RoutineRun {
	out := make([]domain.RoutineRun, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out
}

func (s Routines) CreateRoutineRun(ctx context.Context, rr domain.RoutineRun) (domain.RoutineRun, error) {
	rec := routineRunRecord{
		GuildID: rr.GuildID, RoutineID: rr.RoutineID, TriggerID: nullable(rr.TriggerID), Source: string(rr.Source), Status: string(rr.Status),
		TriggeredAt: rr.TriggeredAt, LinkedIssueID: nullable(rr.LinkedIssueID), CoalescedIntoRoutineRunID: nullable(rr.CoalescedIntoRunID),
		FailureReason: rr.FailureReason, TriggeredByMemberID: nullable(rr.TriggeredBy.MemberID), TriggeredByAgentID: nullable(rr.TriggeredBy.AgentID),
		CompletedAt: rr.CompletedAt,
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.RoutineRun{}, err
	}
	return rec.toDomain(), nil
}

func (s Routines) SaveRoutineRun(ctx context.Context, rr domain.RoutineRun) error {
	_, err := s.query(ctx).Exec(`UPDATE routine_runs SET status = ?, linked_issue_id = ?, coalesced_into_routine_run_id = ?,
		failure_reason = ?, completed_at = ?, updated_at = now() WHERE id = ?`,
		string(rr.Status), nullable(rr.LinkedIssueID), nullable(rr.CoalescedIntoRunID), rr.FailureReason, rr.CompletedAt, rr.ID)
	return err
}

// RoutineRun returns the Routine run; found is false when there is none.
func (s Routines) RoutineRun(ctx context.Context, id uint64) (domain.RoutineRun, bool, error) {
	var rec routineRunRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.RoutineRun{}, false, nil
		}
		return domain.RoutineRun{}, false, err
	}
	return rec.toDomain(), true, nil
}

// RoutineRuns lists the Routine runs of the Routines, newest first, at
// most limit.
func (s Routines) RoutineRuns(ctx context.Context, routineIDs []uint64, limit int) ([]domain.RoutineRun, error) {
	if len(routineIDs) == 0 {
		return nil, nil
	}
	var recs []routineRunRecord
	if err := s.query(ctx).Where("routine_id IN ?", routineIDs).Order("id desc").Limit(limit).Find(&recs); err != nil {
		return nil, err
	}
	return routineRunsOf(recs), nil
}

// LastRoutineRuns answers the newest Routine run of each of the Routines
// that has one.
func (s Routines) LastRoutineRuns(ctx context.Context, routineIDs []uint64) (map[uint64]domain.RoutineRun, error) {
	out := map[uint64]domain.RoutineRun{}
	if len(routineIDs) == 0 {
		return out, nil
	}
	var recs []routineRunRecord
	if err := s.query(ctx).Raw(`SELECT DISTINCT ON (routine_id) * FROM routine_runs WHERE routine_id IN ? ORDER BY routine_id, id DESC`, routineIDs).Scan(&recs); err != nil {
		return nil, err
	}
	for _, rr := range routineRunsOf(recs) {
		out[rr.RoutineID] = rr
	}
	return out, nil
}
