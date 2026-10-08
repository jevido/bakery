package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type runRecord struct {
	ID                  uint64 `gorm:"primaryKey"`
	GuildID             uint64
	AgentID             uint64
	IssueID             *uint64
	InvocationSource    string
	Status              string
	RequestedByMemberID *uint64
	DesktopID           *uint64
	RetryOfRunID        *uint64
	Prompt              string
	SessionID           string
	ExitCode            *int
	Error               string
	InputTokens         int64
	CachedInputTokens   int64
	OutputTokens        int64
	Turns               int64
	CostEquivalentUSD   float64 `gorm:"column:cost_equivalent_usd"`
	DurationMs          int64
	NextSeq             int64
	LeaseExpiresAt      *time.Time
	CreatedAt           time.Time
	StartedAt           *time.Time
	FinishedAt          *time.Time
	UpdatedAt           time.Time
}

func (runRecord) TableName() string { return "runs" }

func (r runRecord) toDomain() domain.Run {
	return domain.Run{
		ID: r.ID, GuildID: r.GuildID, AgentID: r.AgentID, IssueID: deref(r.IssueID),
		InvocationSource: domain.InvocationSource(r.InvocationSource), Status: domain.RunStatus(r.Status),
		RequestedByID: deref(r.RequestedByMemberID), DesktopID: deref(r.DesktopID), RetryOfRunID: deref(r.RetryOfRunID),
		Prompt: r.Prompt, SessionID: r.SessionID, ExitCode: r.ExitCode, Error: r.Error,
		Usage: domain.Usage{
			InputTokens: r.InputTokens, CachedInputTokens: r.CachedInputTokens, OutputTokens: r.OutputTokens,
			Turns: r.Turns, CostEquivalentUSD: r.CostEquivalentUSD, DurationMS: r.DurationMs,
		},
		NextSeq: r.NextSeq, LeaseExpiresAt: utc(r.LeaseExpiresAt), CreatedAt: r.CreatedAt.UTC(),
		StartedAt: utc(r.StartedAt), FinishedAt: utc(r.FinishedAt), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

type runEventRecord struct {
	ID        uint64 `gorm:"primaryKey"`
	RunID     uint64
	Seq       int64
	Kind      string
	Payload   string
	CreatedAt time.Time
}

func (runEventRecord) TableName() string { return "run_events" }

// Runs keeps Runs and their Run events.
type Runs struct{}

func (Runs) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Runs) CreateRun(ctx context.Context, r domain.Run) (domain.Run, error) {
	var recs []runRecord
	err := s.query(ctx).Raw(`INSERT INTO runs (guild_id, agent_id, issue_id, invocation_source, status, requested_by_member_id,
		retry_of_run_id, prompt, next_seq, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *`,
		r.GuildID, r.AgentID, nullable(r.IssueID), string(r.InvocationSource), string(r.Status), nullable(r.RequestedByID),
		nullable(r.RetryOfRunID), r.Prompt, r.NextSeq, r.CreatedAt, r.UpdatedAt).Scan(&recs)
	if err != nil {
		return domain.Run{}, err
	}
	if len(recs) == 0 {
		return domain.Run{}, errors.New("agents: the new run was not returned")
	}
	return recs[0].toDomain(), nil
}

// Run returns the Run; found is false when there is none.
func (s Runs) Run(ctx context.Context, id uint64) (domain.Run, bool, error) {
	var rec runRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Run{}, false, nil
		}
		return domain.Run{}, false, err
	}
	return rec.toDomain(), true, nil
}

// Runs lists the Runs in the query, newest first.
func (s Runs) Runs(ctx context.Context, q app.RunQuery) ([]domain.Run, error) {
	query := s.query(ctx).Where("guild_id", q.GuildID)
	if q.AgentID != 0 {
		query = query.Where("agent_id", q.AgentID)
	}
	if q.IssueID != 0 {
		query = query.Where("issue_id", q.IssueID)
	}
	if len(q.Statuses) > 0 {
		in := make([]any, len(q.Statuses))
		for i, st := range q.Statuses {
			in[i] = string(st)
		}
		query = query.WhereIn("status", in)
	}
	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}
	var recs []runRecord
	if err := query.Order("created_at desc").Order("id desc").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Run, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// SaveRun stores the Run's changes only while it is still in the status
// it was read in, so two requests moving one Run never both win; moved
// is false when another got there first.
func (s Runs) SaveRun(ctx context.Context, r domain.Run, from domain.RunStatus) (moved bool, err error) {
	u := r.Usage
	res, err := s.query(ctx).Exec(`UPDATE runs SET status = ?, desktop_id = ?, session_id = ?, exit_code = ?, error = ?,
		input_tokens = ?, cached_input_tokens = ?, output_tokens = ?, turns = ?, cost_equivalent_usd = ?, duration_ms = ?,
		next_seq = ?, lease_expires_at = ?, started_at = ?, finished_at = ?, updated_at = ?
		WHERE id = ? AND status = ?`,
		string(r.Status), nullable(r.DesktopID), r.SessionID, r.ExitCode, r.Error,
		u.InputTokens, u.CachedInputTokens, u.OutputTokens, u.Turns, u.CostEquivalentUSD, u.DurationMS,
		r.NextSeq, r.LeaseExpiresAt, r.StartedAt, r.FinishedAt, r.UpdatedAt, r.ID, string(from))
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
}

// RunningRuns answers the running Run of each of the Agents that has one.
func (s Runs) RunningRuns(ctx context.Context, agentIDs []uint64) (map[uint64]uint64, error) {
	out := map[uint64]uint64{}
	if len(agentIDs) == 0 {
		return out, nil
	}
	in := make([]any, len(agentIDs))
	for i, id := range agentIDs {
		in[i] = id
	}
	var recs []runRecord
	if err := s.query(ctx).Select("id", "agent_id").Where("status", string(domain.RunRunning)).WhereIn("agent_id", in).Find(&recs); err != nil {
		return nil, err
	}
	for _, r := range recs {
		out[r.AgentID] = r.ID
	}
	return out, nil
}

// RunEvents lists the Run's events after the seq, at most limit, by seq.
func (s Runs) RunEvents(ctx context.Context, runID uint64, after int64, limit int) ([]domain.RunEvent, error) {
	var recs []runEventRecord
	if err := s.query(ctx).Raw(`SELECT id, run_id, seq, kind, payload::text AS payload, created_at FROM run_events
		WHERE run_id = ? AND seq > ? ORDER BY seq LIMIT ?`, runID, after, limit).Scan(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.RunEvent, len(recs))
	for i, r := range recs {
		out[i] = domain.RunEvent{RunID: r.RunID, Seq: r.Seq, Kind: domain.EventKind(r.Kind), Payload: []byte(r.Payload), CreatedAt: r.CreatedAt.UTC()}
	}
	return out, nil
}

// DesktopRuns lists, oldest first, the queued Runs of the Agents the
// Member hired that are not paused or terminated, in every Guild, and the
// running Runs the Desktop holds.
func (s Runs) DesktopRuns(ctx context.Context, memberID, desktopID uint64) ([]domain.Run, error) {
	var recs []runRecord
	err := s.query(ctx).Raw(`SELECT runs.* FROM runs JOIN agents ON agents.id = runs.agent_id
		WHERE agents.hirer_member_id = ?
		AND ((runs.status = ? AND agents.status NOT IN (?, ?)) OR (runs.status = ? AND runs.desktop_id = ?))
		ORDER BY runs.created_at, runs.id`,
		memberID, string(domain.RunQueued), string(domain.Paused), string(domain.Terminated),
		string(domain.RunRunning), desktopID).Scan(&recs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Run, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// AppendRunEvents stores the events, skipping a seq already stored, and
// the Run's NextSeq, Lease and session id in one transaction, only while
// the Run is running with NextSeq still from.
func (s Runs) AppendRunEvents(ctx context.Context, r domain.Run, events []domain.RunEvent, from int64) (bool, error) {
	moved := false
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		res, err := tx.Exec(`UPDATE runs SET next_seq = ?, lease_expires_at = ?, session_id = ?, updated_at = ?
			WHERE id = ? AND status = ? AND next_seq = ?`,
			r.NextSeq, r.LeaseExpiresAt, r.SessionID, r.UpdatedAt, r.ID, string(domain.RunRunning), from)
		if err != nil || res.RowsAffected != 1 {
			return err
		}
		moved = true
		for _, e := range events {
			if _, err := tx.Exec(`INSERT INTO run_events (run_id, seq, kind, payload, created_at)
				VALUES (?, ?, ?, ?::jsonb, ?) ON CONFLICT (run_id, seq) DO NOTHING`,
				e.RunID, e.Seq, string(e.Kind), payloadOf(e.Payload), e.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
	return moved && err == nil, err
}

// payloadOf is a Run event's payload as stored: {} for none.
func payloadOf(p []byte) string {
	if len(p) == 0 {
		return "{}"
	}
	return string(p)
}

// KeepRunLease stores the Run's Lease only while it is running.
func (s Runs) KeepRunLease(ctx context.Context, r domain.Run) (bool, error) {
	res, err := s.query(ctx).Exec(`UPDATE runs SET lease_expires_at = ?, updated_at = ? WHERE id = ? AND status = ?`,
		r.LeaseExpiresAt, r.UpdatedAt, r.ID, string(domain.RunRunning))
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
}

// ExpiredRuns lists the running Runs whose Lease ran out before at.
func (s Runs) ExpiredRuns(ctx context.Context, at time.Time) ([]domain.Run, error) {
	var recs []runRecord
	if err := s.query(ctx).Where("status", string(domain.RunRunning)).Where("lease_expires_at < ?", at).
		Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Run, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}
