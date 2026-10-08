package infra

import (
	"context"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type workProductRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	GuildID           uint64
	IssueID           uint64
	ApplicationID     uint64
	Type              string
	Provider          string
	ExternalID        string
	Title             string
	URL               string `gorm:"column:url"`
	Status            string
	CreatedByRunID    *uint64
	CreatedByAgentID  *uint64
	CreatedByMemberID *uint64
	orm.Timestamps
}

func (workProductRecord) TableName() string { return "issue_work_products" }

func (r workProductRecord) toDomain() domain.WorkProduct {
	w := domain.WorkProduct{
		ID: r.ID, GuildID: r.GuildID, IssueID: r.IssueID, ApplicationID: r.ApplicationID, Type: domain.WorkProductType(r.Type),
		Provider: r.Provider, ExternalID: r.ExternalID, Title: r.Title, URL: r.URL, Status: domain.WorkProductStatus(r.Status),
		CreatedBy: actor(r.CreatedByMemberID, r.CreatedByAgentID),
	}
	if r.CreatedByRunID != nil {
		w.CreatedBy.RunID = *r.CreatedByRunID
	}
	w.CreatedAt, w.UpdatedAt = stamp(&r.Timestamps)
	return w
}

// WorkProducts keeps the Work products of Issues.
type WorkProducts struct{}

func (WorkProducts) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s WorkProducts) WorkProducts(ctx context.Context, issueID uint64) ([]domain.WorkProduct, error) {
	var recs []workProductRecord
	if err := s.query(ctx).Where("issue_id", issueID).Order("created_at desc").Order("id desc").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.WorkProduct, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (s WorkProducts) WorkProduct(ctx context.Context, issueID uint64, typ domain.WorkProductType, externalID string) (domain.WorkProduct, bool, error) {
	var recs []workProductRecord
	if err := s.query(ctx).Where("issue_id", issueID).Where("type", string(typ)).Where("external_id", externalID).Limit(1).Find(&recs); err != nil {
		return domain.WorkProduct{}, false, err
	}
	if len(recs) == 0 {
		return domain.WorkProduct{}, false, nil
	}
	return recs[0].toDomain(), true, nil
}

func (s WorkProducts) ApplicationWorkProducts(ctx context.Context, applicationID uint64, typ domain.WorkProductType, externalID string) ([]domain.WorkProduct, error) {
	var recs []workProductRecord
	if err := s.query(ctx).Where("application_id", applicationID).Where("type", string(typ)).Where("external_id", externalID).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.WorkProduct, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// SaveWorkProduct stores a new Work product, or a known one's title, link
// and status. A new one whose Issue, type and external id another save
// already stored keeps that one, which is answered.
func (s WorkProducts) SaveWorkProduct(ctx context.Context, w domain.WorkProduct) (domain.WorkProduct, error) {
	if w.ID != 0 {
		_, err := s.query(ctx).Exec(`UPDATE issue_work_products SET title = ?, url = ?, status = ?, updated_at = now() WHERE id = ?`,
			w.Title, w.URL, string(w.Status), w.ID)
		if err != nil {
			return domain.WorkProduct{}, err
		}
	} else {
		var run *uint64
		if w.CreatedBy.AgentID != 0 {
			run = nullable(w.CreatedBy.RunID)
		}
		_, err := s.query(ctx).Exec(`
			INSERT INTO issue_work_products (guild_id, issue_id, application_id, type, provider, external_id, title, url, status,
				created_by_run_id, created_by_agent_id, created_by_member_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now(), now())
			ON CONFLICT (issue_id, type, external_id) DO NOTHING`,
			w.GuildID, w.IssueID, w.ApplicationID, string(w.Type), w.Provider, w.ExternalID, w.Title, w.URL, string(w.Status),
			run, nullable(w.CreatedBy.AgentID), nullable(w.CreatedBy.MemberID))
		if err != nil {
			return domain.WorkProduct{}, err
		}
	}
	saved, _, err := s.WorkProduct(ctx, w.IssueID, w.Type, w.ExternalID)
	return saved, err
}
