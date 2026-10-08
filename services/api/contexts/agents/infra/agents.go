// Package infra is the agents context's persistence.
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
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type agentRecord struct {
	ID             uint64 `gorm:"primaryKey"`
	GuildID        uint64
	HirerMemberID  uint64
	Name           string
	Job            string
	Title          string
	Icon           string
	Capabilities   string
	ManagerAgentID *uint64
	Status         string
	HireApprovalID *uint64
	// The Heartbeat policy; the columns are named as the policy reads.
	HeartbeatEnabled     bool
	HeartbeatIntervalSec int
	WakeOnDemand         bool
	LastHeartbeatAt      *time.Time
	PausedAt             *time.Time
	TerminatedAt         *time.Time
	orm.Timestamps
}

func (agentRecord) TableName() string { return "agents" }

// nullable stores 0 as NULL.
func nullable(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

func deref(id *uint64) uint64 {
	if id == nil {
		return 0
	}
	return *id
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (r agentRecord) toDomain() domain.Agent {
	a := domain.Agent{
		ID: r.ID, GuildID: r.GuildID, HirerID: r.HirerMemberID, Name: r.Name, Job: domain.Job(r.Job), Title: r.Title,
		Icon: domain.Icon(r.Icon), Capabilities: r.Capabilities, ManagerID: deref(r.ManagerAgentID), Status: domain.Status(r.Status),
		HireApprovalID: deref(r.HireApprovalID), PausedAt: utc(r.PausedAt), TerminatedAt: utc(r.TerminatedAt),
		Heartbeat:       domain.HeartbeatPolicy{Enabled: r.HeartbeatEnabled, IntervalSec: r.HeartbeatIntervalSec, WakeOnDemand: r.WakeOnDemand},
		LastHeartbeatAt: utc(r.LastHeartbeatAt),
	}
	if r.CreatedAt != nil {
		a.CreatedAt = r.CreatedAt.StdTime().UTC()
	}
	if r.UpdatedAt != nil {
		a.UpdatedAt = r.UpdatedAt.StdTime().UTC()
	}
	return a
}

// Agents keeps Agents.
type Agents struct{}

func (Agents) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Agents) Agents(ctx context.Context, guildID uint64) ([]domain.Agent, error) {
	var recs []agentRecord
	if err := s.query(ctx).Where("guild_id", guildID).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Agent, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// Agent returns the Agent; found is false when there is none.
func (s Agents) Agent(ctx context.Context, id uint64) (domain.Agent, bool, error) {
	var rec agentRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Agent{}, false, nil
		}
		return domain.Agent{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Agents) CreateAgent(ctx context.Context, a domain.Agent) (domain.Agent, error) {
	rec := agentRecord{
		GuildID: a.GuildID, HirerMemberID: a.HirerID, Name: a.Name, Job: string(a.Job), Title: a.Title, Icon: string(a.Icon),
		Capabilities: a.Capabilities, ManagerAgentID: nullable(a.ManagerID), Status: string(a.Status),
		HireApprovalID: nullable(a.HireApprovalID), PausedAt: a.PausedAt, TerminatedAt: a.TerminatedAt,
		HeartbeatEnabled: a.Heartbeat.Enabled, HeartbeatIntervalSec: a.Heartbeat.IntervalSec, WakeOnDemand: a.Heartbeat.WakeOnDemand,
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		if taken(err) {
			return domain.Agent{}, domain.ErrNameTaken
		}
		return domain.Agent{}, err
	}
	return rec.toDomain(), nil
}

// SaveAgent stores the Agent but not LastHeartbeatAt, which only the
// timer's conditional claim writes.
func (s Agents) SaveAgent(ctx context.Context, a domain.Agent) error {
	_, err := s.query(ctx).Exec(`UPDATE agents SET name = ?, job = ?, title = ?, icon = ?, capabilities = ?, manager_agent_id = ?,
		status = ?, hire_approval_id = ?, heartbeat_enabled = ?, heartbeat_interval_sec = ?, wake_on_demand = ?,
		paused_at = ?, terminated_at = ?, updated_at = now() WHERE id = ?`,
		a.Name, string(a.Job), a.Title, string(a.Icon), a.Capabilities, nullable(a.ManagerID),
		string(a.Status), nullable(a.HireApprovalID), a.Heartbeat.Enabled, a.Heartbeat.IntervalSec, a.Heartbeat.WakeOnDemand,
		a.PausedAt, a.TerminatedAt, a.ID)
	if taken(err) {
		return domain.ErrNameTaken
	}
	return err
}

func (s Agents) SaveHeartbeat(ctx context.Context, a domain.Agent) error {
	_, err := s.query(ctx).Exec(`UPDATE agents SET heartbeat_enabled = ?, heartbeat_interval_sec = ?, wake_on_demand = ?,
		updated_at = now() WHERE id = ?`, a.Heartbeat.Enabled, a.Heartbeat.IntervalSec, a.Heartbeat.WakeOnDemand, a.ID)
	return err
}

func (s Agents) DueHeartbeats(ctx context.Context, now time.Time) ([]domain.Agent, error) {
	var recs []agentRecord
	err := s.query(ctx).Raw(`SELECT * FROM agents WHERE heartbeat_enabled AND status IN ('idle', 'running', 'error')
		AND COALESCE(last_heartbeat_at, created_at) + make_interval(secs => heartbeat_interval_sec) <= ? ORDER BY id`, now).Scan(&recs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Agent, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// ClaimHeartbeat is Paperclip's claimDueTimerHeartbeat: the write only hits
// a row whose last_heartbeat_at is still the one read, so of two API
// processes ticking at once only one queues the timer Run.
func (s Agents) ClaimHeartbeat(ctx context.Context, id uint64, seen *time.Time, at time.Time) (bool, error) {
	res, err := s.query(ctx).Exec(`UPDATE agents SET last_heartbeat_at = ? WHERE id = ? AND last_heartbeat_at IS NOT DISTINCT FROM ?`, at, id, seen)
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
}

func (s Agents) DeleteAgent(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM agents WHERE id = ?`, id)
	return err
}

// taken reports the unique name index refusing a row.
func taken(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
