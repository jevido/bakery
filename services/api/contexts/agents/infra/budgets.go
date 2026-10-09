package infra

import (
	"context"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type budgetRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	GuildID           uint64
	ScopeType         string
	ScopeID           uint64
	Metric            string
	WindowKind        string
	Amount            int64
	WarnPercent       int
	HardStop          bool
	Notify            bool
	CreatedByMemberID *uint64
	UpdatedByMemberID *uint64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (budgetRecord) TableName() string { return "budgets" }

func (r budgetRecord) toDomain() domain.Budget {
	return domain.Budget{
		ID: r.ID, GuildID: r.GuildID, Scope: domain.BudgetScope(r.ScopeType), ScopeID: r.ScopeID,
		Metric: domain.BudgetMetric(r.Metric), Window: domain.BudgetWindow(r.WindowKind), Amount: r.Amount,
		WarnPercent: r.WarnPercent, HardStop: r.HardStop, Notify: r.Notify,
		CreatedBy: deref(r.CreatedByMemberID), UpdatedBy: deref(r.UpdatedByMemberID), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// Budgets keeps Budgets.
type Budgets struct{}

func (Budgets) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Budgets) Budgets(ctx context.Context, guildID uint64) ([]domain.Budget, error) {
	var recs []budgetRecord
	if err := s.query(ctx).Where("guild_id", guildID).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Budget, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// SaveBudget upserts on the Budget's Guild, scope, metric and window, so
// two people setting the same Budget at once both end in one row.
func (s Budgets) SaveBudget(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	var recs []budgetRecord
	err := s.query(ctx).Raw(`INSERT INTO budgets (guild_id, scope_type, scope_id, metric, window_kind, amount, warn_percent,
			hard_stop, notify, created_by_member_id, updated_by_member_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (guild_id, scope_type, scope_id, metric, window_kind) DO UPDATE SET amount = EXCLUDED.amount,
			warn_percent = EXCLUDED.warn_percent, hard_stop = EXCLUDED.hard_stop, notify = EXCLUDED.notify,
			updated_by_member_id = EXCLUDED.updated_by_member_id, updated_at = EXCLUDED.updated_at
		RETURNING *`,
		b.GuildID, string(b.Scope), b.ScopeID, string(b.Metric), string(b.Window), b.Amount, b.WarnPercent,
		b.HardStop, b.Notify, nullable(b.CreatedBy), nullable(b.UpdatedBy), b.CreatedAt, b.UpdatedAt).Scan(&recs)
	if err != nil || len(recs) == 0 {
		return domain.Budget{}, err
	}
	return recs[0].toDomain(), nil
}

func (s Budgets) DeleteBudgetsOf(ctx context.Context, scope domain.BudgetScope, scopeID uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM budgets WHERE scope_type = ? AND scope_id = ?`, string(scope), scopeID)
	return err
}

type incidentRecord struct {
	ID             uint64 `gorm:"primaryKey"`
	GuildID        uint64
	BudgetID       uint64
	ScopeType      string
	ScopeID        uint64
	Metric         string
	WindowKind     string
	WindowStart    *time.Time
	WindowEnd      *time.Time
	Threshold      string
	AmountLimit    int64
	AmountObserved int64
	Status         string
	ApprovalID     *uint64
	ResolvedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (incidentRecord) TableName() string { return "budget_incidents" }

func (r incidentRecord) toDomain() domain.BudgetIncident {
	i := domain.BudgetIncident{
		ID: r.ID, GuildID: r.GuildID, BudgetID: r.BudgetID, Scope: domain.BudgetScope(r.ScopeType), ScopeID: r.ScopeID,
		Metric: domain.BudgetMetric(r.Metric), Window: domain.BudgetWindow(r.WindowKind), Threshold: domain.Threshold(r.Threshold),
		Amount: r.AmountLimit, Observed: r.AmountObserved, Status: domain.IncidentStatus(r.Status), ApprovalID: deref(r.ApprovalID),
		ResolvedAt: utc(r.ResolvedAt), CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.WindowStart != nil {
		i.WindowStart, i.WindowEnd = r.WindowStart.UTC(), r.WindowEnd.UTC()
	}
	return i
}

// bound stores a lifetime window's zero bound as NULL.
func bound(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s Budgets) BudgetIncidents(ctx context.Context, guildID, budgetID uint64, statuses []domain.IncidentStatus) ([]domain.BudgetIncident, error) {
	q := s.query(ctx).Where("guild_id", guildID)
	if budgetID != 0 {
		q = q.Where("budget_id", budgetID)
	}
	if len(statuses) > 0 {
		in := make([]any, len(statuses))
		for i, st := range statuses {
			in[i] = string(st)
		}
		q = q.WhereIn("status", in)
	}
	var recs []incidentRecord
	if err := q.Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.BudgetIncident, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (s Budgets) OpenIncidentsOf(ctx context.Context, scope domain.BudgetScope, scopeID uint64) ([]domain.BudgetIncident, error) {
	var recs []incidentRecord
	if err := s.query(ctx).Where("scope_type", string(scope)).Where("scope_id", scopeID).Where("status", string(domain.IncidentOpen)).
		Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.BudgetIncident, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// OpenIncident inserts the incident unless one not dismissed already
// holds its Budget, window and threshold: the unique index settles two
// Runs finishing at once.
func (s Budgets) OpenIncident(ctx context.Context, i domain.BudgetIncident) (domain.BudgetIncident, bool, error) {
	var recs []incidentRecord
	err := s.query(ctx).Raw(`INSERT INTO budget_incidents (guild_id, budget_id, scope_type, scope_id, metric, window_kind, window_start,
			window_end, threshold, amount_limit, amount_observed, status, approval_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (budget_id, window_start, threshold) WHERE status <> 'dismissed' DO NOTHING
		RETURNING *`,
		i.GuildID, i.BudgetID, string(i.Scope), i.ScopeID, string(i.Metric), string(i.Window), bound(i.WindowStart),
		bound(i.WindowEnd), string(i.Threshold), i.Amount, i.Observed, string(i.Status), nullable(i.ApprovalID), i.CreatedAt, i.UpdatedAt).Scan(&recs)
	if err != nil || len(recs) == 0 {
		return domain.BudgetIncident{}, false, err
	}
	return recs[0].toDomain(), true, nil
}

func (s Budgets) SaveIncident(ctx context.Context, i domain.BudgetIncident) (bool, error) {
	res, err := s.query(ctx).Exec(`UPDATE budget_incidents SET status = ?, approval_id = ?, resolved_at = ?, updated_at = ?
		WHERE id = ? AND status = 'open'`,
		string(i.Status), nullable(i.ApprovalID), i.ResolvedAt, i.UpdatedAt, i.ID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
}
