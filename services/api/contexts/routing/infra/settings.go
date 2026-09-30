package infra

import (
	"context"

	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

type settingsRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	WwwRedirect   string
	orm.Timestamps
}

func (settingsRecord) TableName() string { return "route_settings" }

func (r settingsRecord) toDomain() domain.RouteSettings {
	return domain.RouteSettings{ApplicationID: r.ApplicationID, WwwRedirect: domain.WwwRedirect(r.WwwRedirect)}
}

// Settings stores Route settings, one row per Application.
type Settings struct{}

// All returns every stored Route settings by application id.
func (Settings) All(ctx context.Context) (map[uint64]domain.RouteSettings, error) {
	var recs []settingsRecord
	if err := facades.Orm().WithContext(ctx).Query().Find(&recs); err != nil {
		return nil, err
	}
	out := make(map[uint64]domain.RouteSettings, len(recs))
	for _, r := range recs {
		out[r.ApplicationID] = r.toDomain()
	}
	return out, nil
}

// Get returns the Application's Route settings; found is false when none
// are stored.
func (Settings) Get(ctx context.Context, applicationID uint64) (domain.RouteSettings, bool, error) {
	var recs []settingsRecord
	if err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Find(&recs); err != nil || len(recs) == 0 {
		return domain.RouteSettings{}, false, err
	}
	return recs[0].toDomain(), true, nil
}

func (Settings) Put(ctx context.Context, s domain.RouteSettings) error {
	_, err := facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO route_settings (application_id, www_redirect, created_at, updated_at)
		VALUES (?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE
		SET www_redirect = EXCLUDED.www_redirect, updated_at = now()`,
		s.ApplicationID, string(s.WwwRedirect))
	return err
}

func (Settings) Delete(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&settingsRecord{})
	return err
}
