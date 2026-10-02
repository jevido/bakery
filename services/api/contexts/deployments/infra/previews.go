package infra

import (
	"context"
	"time"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type previewRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	Number        int
	Branch        string
	Title         string
	Url           string
	Provider      string
	Api           string
	State         string
	CommentID     string
	ClosedAt      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (previewRecord) TableName() string { return "previews" }

func (r previewRecord) toDomain() domain.Preview {
	return domain.Preview{
		ID: r.ID, ApplicationID: r.ApplicationID, Number: r.Number, Branch: r.Branch, Title: r.Title, URL: r.Url,
		Provider: domain.Provider(r.Provider), API: r.Api, State: domain.PreviewState(r.State), CommentID: r.CommentID,
		CreatedAt: r.CreatedAt, ClosedAt: r.ClosedAt,
	}
}

// Previews stores Previews, one per Application and number.
type Previews struct{}

// Save creates or replaces the Preview of (application, number).
func (Previews) Save(ctx context.Context, p domain.Preview) (domain.Preview, error) {
	var recs []previewRecord
	err := facades.Orm().WithContext(ctx).Query().Raw(`
		INSERT INTO previews (application_id, number, branch, title, url, provider, api, state, comment_id, closed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id, number) DO UPDATE
		SET branch = EXCLUDED.branch, title = EXCLUDED.title, url = EXCLUDED.url, provider = EXCLUDED.provider,
		    api = EXCLUDED.api, state = EXCLUDED.state, comment_id = EXCLUDED.comment_id,
		    closed_at = EXCLUDED.closed_at, updated_at = now()
		RETURNING *`,
		p.ApplicationID, p.Number, p.Branch, p.Title, p.URL, string(p.Provider), p.API, string(p.State), p.CommentID, p.ClosedAt).Scan(&recs)
	if err != nil || len(recs) == 0 {
		return domain.Preview{}, err
	}
	return recs[0].toDomain(), nil
}

func (Previews) ByNumber(ctx context.Context, applicationID uint64, number int) (domain.Preview, bool, error) {
	var recs []previewRecord
	if err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Where("number", number).Limit(1).Find(&recs); err != nil {
		return domain.Preview{}, false, err
	}
	if len(recs) == 0 {
		return domain.Preview{}, false, nil
	}
	return recs[0].toDomain(), true, nil
}

// ByApplication lists the Application's Previews, open ones first, then
// newest first.
func (Previews) ByApplication(ctx context.Context, applicationID uint64) ([]domain.Preview, error) {
	var recs []previewRecord
	if err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).
		OrderByRaw("state = 'open' DESC, number DESC").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Preview, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (Previews) DeleteForApplication(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&previewRecord{})
	return err
}
