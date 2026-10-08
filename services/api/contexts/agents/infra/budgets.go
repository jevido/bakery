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
