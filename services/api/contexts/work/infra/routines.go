package infra

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type routineRecord struct {
	ID                   uint64 `gorm:"primaryKey"`
	GuildID              uint64
	ProjectID            *uint64
	GoalID               *uint64
	ParentIssueID        *uint64
	Title                string
	Description          string
	AssigneeAgentID      *uint64
	Priority             string
	Status               string
	ConcurrencyPolicy    string
	CatchUpPolicy        string
	Variables            string `gorm:"type:jsonb"`
	CreatedByMemberID    *uint64
	CreatedByAgentID     *uint64
	LastTriggeredAt      *time.Time
	LatestRevisionID     *uint64
	LatestRevisionNumber int
	orm.Timestamps
}

func (routineRecord) TableName() string { return "routines" }

func (r routineRecord) toDomain() domain.Routine {
	out := domain.Routine{
		ID: r.ID, GuildID: r.GuildID, ProjectID: deref(r.ProjectID), GoalID: deref(r.GoalID), ParentIssueID: deref(r.ParentIssueID),
		AssigneeAgentID: deref(r.AssigneeAgentID), Title: r.Title, Description: r.Description,
		Priority: domain.Priority(r.Priority), Status: domain.RoutineStatus(r.Status),
		ConcurrencyPolicy: domain.ConcurrencyPolicy(r.ConcurrencyPolicy), CatchUpPolicy: domain.CatchUpPolicy(r.CatchUpPolicy),
		CreatedBy: actor(r.CreatedByMemberID, r.CreatedByAgentID), Variables: variablesOf(r.Variables),
		LatestRevisionID: deref(r.LatestRevisionID), LatestRevisionNumber: r.LatestRevisionNumber,
	}
	if r.LastTriggeredAt != nil {
		t := r.LastTriggeredAt.UTC()
		out.LastTriggeredAt = &t
	}
	out.CreatedAt, out.UpdatedAt = stamp(&r.Timestamps)
	return out
}

// variableRecord is a Routine variable as routines.variables keeps it.
type variableRecord struct {
	Name     string   `json:"name"`
	Label    string   `json:"label,omitempty"`
	Type     string   `json:"type"`
	Default  any      `json:"default_value"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
}

func variablesOf(column string) []domain.RoutineVariable {
	var recs []variableRecord
	_ = json.Unmarshal([]byte(column), &recs)
	out := make([]domain.RoutineVariable, len(recs))
	for n, v := range recs {
		out[n] = domain.RoutineVariable{Name: v.Name, Label: v.Label, Type: domain.VariableType(v.Type), Default: v.Default, Required: v.Required, Options: v.Options}
		if out[n].Options == nil {
			out[n].Options = []string{}
		}
	}
	return out
}

func variablesColumn(vars []domain.RoutineVariable) string {
	recs := make([]variableRecord, len(vars))
	for n, v := range vars {
		recs[n] = variableRecord{Name: v.Name, Label: v.Label, Type: string(v.Type), Default: v.Default, Required: v.Required, Options: v.Options}
	}
	b, _ := json.Marshal(recs)
	return string(b)
}

// valuesColumn is a Routine run's variables' values as
// routine_runs.variables keeps them; nil for none.
func valuesColumn(values map[string]any) *string {
	if values == nil {
		return nil
	}
	b, _ := json.Marshal(values)
	s := string(b)
	return &s
}

func valuesOf(column *string) map[string]any {
	if column == nil {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(*column), &out)
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
		ConcurrencyPolicy: string(r.ConcurrencyPolicy), CatchUpPolicy: string(r.CatchUpPolicy), Variables: variablesColumn(r.Variables),
		CreatedByMemberID: nullable(r.CreatedBy.MemberID), CreatedByAgentID: nullable(r.CreatedBy.AgentID),
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Routine{}, err
	}
	return rec.toDomain(), nil
}

func (s Routines) SaveRoutine(ctx context.Context, r domain.Routine) error {
	_, err := s.query(ctx).Exec(`UPDATE routines SET project_id = ?, goal_id = ?, parent_issue_id = ?, title = ?, description = ?,
		assignee_agent_id = ?, priority = ?, status = ?, concurrency_policy = ?, catch_up_policy = ?, variables = ?::jsonb, updated_at = now() WHERE id = ?`,
		nullable(r.ProjectID), nullable(r.GoalID), nullable(r.ParentIssueID), r.Title, r.Description,
		nullable(r.AssigneeAgentID), string(r.Priority), string(r.Status), string(r.ConcurrencyPolicy), string(r.CatchUpPolicy),
		variablesColumn(r.Variables), r.ID)
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
	ID                 uint64 `gorm:"primaryKey"`
	GuildID            uint64
	RoutineID          uint64
	Kind               string
	Label              string
	Enabled            bool
	CronExpression     string
	Timezone           string
	NextRunAt          *time.Time
	LastFiredAt        *time.Time
	LastResult         string
	PublicID           *string
	SecretEncrypted    *string
	SigningMode        *string
	ReplayWindowSec    *int
	LastRotatedAt      *time.Time
	LastDeliveryStatus *string
	LastDeliveryAt     *time.Time
	CreatedByMemberID  *uint64
	CreatedByAgentID   *uint64
	orm.Timestamps
}

func (routineTriggerRecord) TableName() string { return "routine_triggers" }

func (r routineTriggerRecord) toDomain() (domain.RoutineTrigger, error) {
	out := domain.RoutineTrigger{
		ID: r.ID, GuildID: r.GuildID, RoutineID: r.RoutineID, Kind: domain.TriggerKind(r.Kind), Label: r.Label, Enabled: r.Enabled,
		CronExpression: r.CronExpression, Timezone: r.Timezone, NextRunAt: utc(r.NextRunAt), LastFiredAt: utc(r.LastFiredAt),
		LastResult: r.LastResult, PublicID: orZero(r.PublicID), SigningMode: domain.SigningMode(orZero(r.SigningMode)),
		ReplayWindowSec: orZero(r.ReplayWindowSec), LastRotatedAt: utc(r.LastRotatedAt),
		CreatedBy: actor(r.CreatedByMemberID, r.CreatedByAgentID),
	}
	if r.SecretEncrypted != nil {
		secret, err := facades.Crypt().DecryptString(*r.SecretEncrypted)
		if err != nil {
			return domain.RoutineTrigger{}, err
		}
		out.Secret = secret
	}
	if r.LastDeliveryStatus != nil && r.LastDeliveryAt != nil {
		out.LastDelivery = &domain.WebhookDelivery{Status: domain.DeliveryStatus(*r.LastDeliveryStatus), ReceivedAt: r.LastDeliveryAt.UTC()}
	}
	out.CreatedAt, out.UpdatedAt = stamp(&r.Timestamps)
	return out, nil
}

func triggersOf(recs []routineTriggerRecord) ([]domain.RoutineTrigger, error) {
	out := make([]domain.RoutineTrigger, len(recs))
	for i, r := range recs {
		t, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = t
	}
	return out, nil
}

// webhookColumns is what a Webhook trigger keeps beyond any Routine
// trigger, its secret encrypted; all null on another kind.
func webhookColumns(t domain.RoutineTrigger) (publicID, secret, mode *string, window *int, deliveryStatus *string, deliveryAt *time.Time, err error) {
	if t.Kind != domain.WebhookTrigger {
		return nil, nil, nil, nil, nil, nil, nil
	}
	enc, err := facades.Crypt().EncryptString(t.Secret)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	m := string(t.SigningMode)
	if t.LastDelivery != nil {
		st, at := string(t.LastDelivery.Status), t.LastDelivery.ReceivedAt
		deliveryStatus, deliveryAt = &st, &at
	}
	return &t.PublicID, &enc, &m, &t.ReplayWindowSec, deliveryStatus, deliveryAt, nil
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
	return triggersOf(recs)
}

// Trigger returns the Routine trigger; found is false when there is none.
func (s Routines) Trigger(ctx context.Context, id uint64) (domain.RoutineTrigger, bool, error) {
	return s.triggerWhere(ctx, "id", id)
}

// TriggerByPublicID returns the Webhook trigger with the Public id; found
// is false when there is none.
func (s Routines) TriggerByPublicID(ctx context.Context, publicID string) (domain.RoutineTrigger, bool, error) {
	return s.triggerWhere(ctx, "public_id", publicID)
}

func (s Routines) triggerWhere(ctx context.Context, column string, v any) (domain.RoutineTrigger, bool, error) {
	var rec routineTriggerRecord
	if err := s.query(ctx).Where(column, v).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.RoutineTrigger{}, false, nil
		}
		return domain.RoutineTrigger{}, false, err
	}
	t, err := rec.toDomain()
	return t, err == nil, err
}

func (s Routines) CreateTrigger(ctx context.Context, t domain.RoutineTrigger) (domain.RoutineTrigger, error) {
	publicID, secret, mode, window, deliveryStatus, deliveryAt, err := webhookColumns(t)
	if err != nil {
		return domain.RoutineTrigger{}, err
	}
	rec := routineTriggerRecord{
		GuildID: t.GuildID, RoutineID: t.RoutineID, Kind: string(t.Kind), Label: t.Label, Enabled: t.Enabled,
		CronExpression: t.CronExpression, Timezone: t.Timezone, NextRunAt: t.NextRunAt,
		PublicID: publicID, SecretEncrypted: secret, SigningMode: mode, ReplayWindowSec: window,
		LastRotatedAt: t.LastRotatedAt, LastDeliveryStatus: deliveryStatus, LastDeliveryAt: deliveryAt,
		CreatedByMemberID: nullable(t.CreatedBy.MemberID), CreatedByAgentID: nullable(t.CreatedBy.AgentID),
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.RoutineTrigger{}, err
	}
	return rec.toDomain()
}

func (s Routines) SaveTrigger(ctx context.Context, t domain.RoutineTrigger) error {
	publicID, secret, mode, window, deliveryStatus, deliveryAt, err := webhookColumns(t)
	if err != nil {
		return err
	}
	_, err = s.query(ctx).Exec(`UPDATE routine_triggers SET label = ?, enabled = ?, cron_expression = ?, timezone = ?, next_run_at = ?,
		last_fired_at = ?, last_result = ?, public_id = ?, secret_encrypted = ?, signing_mode = ?, replay_window_sec = ?,
		last_rotated_at = ?, last_delivery_status = ?, last_delivery_at = ?, updated_at = now() WHERE id = ?`,
		t.Label, t.Enabled, t.CronExpression, t.Timezone, t.NextRunAt, t.LastFiredAt, t.LastResult,
		publicID, secret, mode, window, t.LastRotatedAt, deliveryStatus, deliveryAt, t.ID)
	return err
}

func (s Routines) DeleteTrigger(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM routine_triggers WHERE id = ?`, id)
	return err
}

func (s Routines) DueTriggers(ctx context.Context, now time.Time) ([]domain.RoutineTrigger, error) {
	var recs []routineTriggerRecord
	if err := s.query(ctx).Raw(`SELECT t.* FROM routine_triggers t JOIN routines r ON r.id = t.routine_id
		WHERE t.kind = 'schedule' AND t.enabled AND t.next_run_at IS NOT NULL AND t.next_run_at <= ?
		AND r.status = 'active' AND r.assignee_agent_id IS NOT NULL
		ORDER BY t.next_run_at, t.created_at, t.id`, now).Scan(&recs); err != nil {
		return nil, err
	}
	return triggersOf(recs)
}

// ClaimTrigger is Paperclip's claim in tickScheduledTriggers: the write
// only hits a row whose next_run_at is still the one read, so of two API
// processes ticking at once only one fires the trigger.
func (s Routines) ClaimTrigger(ctx context.Context, id uint64, seen, next time.Time) (bool, error) {
	res, err := s.query(ctx).Exec(`UPDATE routine_triggers SET next_run_at = ?, updated_at = now() WHERE id = ? AND enabled AND next_run_at = ?`, next, id, seen)
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
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
	IdempotencyKey            *string
	Variables                 *string `gorm:"type:jsonb"`
	RoutineRevisionID         *uint64
	CompletedAt               *time.Time
	orm.Timestamps
}

func (routineRunRecord) TableName() string { return "routine_runs" }

func (r routineRunRecord) toDomain() domain.RoutineRun {
	out := domain.RoutineRun{
		ID: r.ID, GuildID: r.GuildID, RoutineID: r.RoutineID, TriggerID: deref(r.TriggerID),
		Source: domain.RoutineRunSource(r.Source), Status: domain.RoutineRunStatus(r.Status), TriggeredAt: r.TriggeredAt.UTC(),
		LinkedIssueID: deref(r.LinkedIssueID), CoalescedIntoRunID: deref(r.CoalescedIntoRoutineRunID), FailureReason: r.FailureReason,
		TriggeredBy: actor(r.TriggeredByMemberID, r.TriggeredByAgentID), IdempotencyKey: orZero(r.IdempotencyKey), CompletedAt: utc(r.CompletedAt),
		Variables: valuesOf(r.Variables), RoutineRevisionID: deref(r.RoutineRevisionID),
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
		IdempotencyKey: nullableString(rr.IdempotencyKey), Variables: valuesColumn(rr.Variables), RoutineRevisionID: nullable(rr.RoutineRevisionID),
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
	return s.routineRunWhere(ctx, "id = ?", id)
}

// RoutineRunByIdempotencyKey returns the trigger's Routine run with the
// Idempotency key; found is false when there is none.
func (s Routines) RoutineRunByIdempotencyKey(ctx context.Context, triggerID uint64, key string) (domain.RoutineRun, bool, error) {
	return s.routineRunWhere(ctx, "trigger_id = ? AND idempotency_key = ?", triggerID, key)
}

func (s Routines) routineRunWhere(ctx context.Context, where string, args ...any) (domain.RoutineRun, bool, error) {
	var rec routineRunRecord
	if err := s.query(ctx).Where(where, args...).FirstOrFail(&rec); err != nil {
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

type routineRevisionRecord struct {
	ID                     uint64 `gorm:"primaryKey"`
	GuildID                uint64
	RoutineID              uint64
	RevisionNumber         int
	Title                  string
	Description            string
	Snapshot               string `gorm:"type:jsonb"`
	ChangeSummary          *string
	RestoredFromRevisionID *uint64
	CreatedByMemberID      *uint64
	CreatedByAgentID       *uint64
	CreatedAt              time.Time
}

func (routineRevisionRecord) TableName() string { return "routine_revisions" }

func (r routineRevisionRecord) toDomain() (domain.RoutineRevision, error) {
	out := domain.RoutineRevision{
		ID: r.ID, GuildID: r.GuildID, RoutineID: r.RoutineID, Number: r.RevisionNumber, Title: r.Title, Description: r.Description,
		ChangeSummary: orZero(r.ChangeSummary), RestoredFromID: deref(r.RestoredFromRevisionID),
		Author: actor(r.CreatedByMemberID, r.CreatedByAgentID), CreatedAt: r.CreatedAt.UTC(),
	}
	if err := json.Unmarshal([]byte(r.Snapshot), &out.Snapshot); err != nil {
		return domain.RoutineRevision{}, err
	}
	return out, nil
}

// AppendRevision adds the Routine revision after the Routine's newest one,
// of the Routine and its triggers as they are now. The Routine's row is
// locked while it does, so two appends at once take turns and the numbers
// have no gaps.
func (s Routines) AppendRevision(ctx context.Context, routineID uint64, by domain.Actor, changeSummary string, restoredFromID uint64) (domain.RoutineRevision, error) {
	var out domain.RoutineRevision
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		var recs []routineRecord
		if err := tx.Raw(`SELECT * FROM routines WHERE id = ? FOR UPDATE`, routineID).Scan(&recs); err != nil {
			return err
		}
		if len(recs) == 0 {
			return app.ErrNotFound
		}
		var trecs []routineTriggerRecord
		if err := tx.Where("routine_id", routineID).Order("id").Find(&trecs); err != nil {
			return err
		}
		ts, err := triggersOf(trecs)
		if err != nil {
			return err
		}
		rev := domain.NewRoutineRevision(recs[0].toDomain(), ts, by, changeSummary, restoredFromID, time.Time{})
		snapshot, err := json.Marshal(rev.Snapshot)
		if err != nil {
			return err
		}
		var rows []routineRevisionRecord
		if err := tx.Raw(`INSERT INTO routine_revisions (guild_id, routine_id, revision_number, title, description, snapshot, change_summary,
			restored_from_revision_id, created_by_member_id, created_by_agent_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?::jsonb, ?, ?, ?, ?, now()) RETURNING *`,
			rev.GuildID, rev.RoutineID, rev.Number, rev.Title, rev.Description, string(snapshot), nullableString(changeSummary),
			nullable(restoredFromID), nullable(by.MemberID), nullable(by.AgentID)).Scan(&rows); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE routines SET latest_revision_id = ?, latest_revision_number = ? WHERE id = ?`, rows[0].ID, rev.Number, routineID); err != nil {
			return err
		}
		out, err = rows[0].toDomain()
		return err
	})
	return out, err
}

// RoutineRevisions lists the Routine's revisions, newest first, at most
// limit.
func (s Routines) RoutineRevisions(ctx context.Context, routineID uint64, limit int) ([]domain.RoutineRevision, error) {
	var recs []routineRevisionRecord
	if err := s.query(ctx).Where("routine_id", routineID).Order("revision_number desc").Limit(limit).Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.RoutineRevision, len(recs))
	for i, r := range recs {
		rev, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = rev
	}
	return out, nil
}

// RoutineRevision returns the Routine's revision; found is false when it
// has none by that id.
func (s Routines) RoutineRevision(ctx context.Context, routineID, id uint64) (domain.RoutineRevision, bool, error) {
	var rec routineRevisionRecord
	if err := s.query(ctx).Where("id", id).Where("routine_id", routineID).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.RoutineRevision{}, false, nil
		}
		return domain.RoutineRevision{}, false, err
	}
	rev, err := rec.toDomain()
	return rev, err == nil, err
}

// nullableString keeps "" as null.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// orZero reads a nullable column; null is the zero value.
func orZero[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
