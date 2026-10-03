package infra

import (
	"context"
	"encoding/json"

	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

type settingsRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	Redirect      string
	// ResponseHeaders is a JSON array of {name, value}.
	ResponseHeaders       string
	BasicAuthEnabled      bool
	BasicAuthUsername     string
	BasicAuthPasswordHash string
	orm.Timestamps
}

type headerJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (settingsRecord) TableName() string { return "route_settings" }

func (r settingsRecord) toDomain() (domain.RouteSettings, error) {
	var headers []headerJSON
	if err := json.Unmarshal([]byte(r.ResponseHeaders), &headers); err != nil {
		return domain.RouteSettings{}, err
	}
	s := domain.RouteSettings{
		ApplicationID: r.ApplicationID, Redirect: domain.Redirect(r.Redirect),
		ResponseHeaders: make([]domain.ResponseHeader, len(headers)),
		BasicAuth:       domain.BasicAuth{Enabled: r.BasicAuthEnabled, Username: r.BasicAuthUsername, PasswordHash: r.BasicAuthPasswordHash},
	}
	for i, h := range headers {
		s.ResponseHeaders[i] = domain.ResponseHeader(h)
	}
	return s, nil
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
		s, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[r.ApplicationID] = s
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
	s, err := recs[0].toDomain()
	return s, err == nil, err
}

func (Settings) Put(ctx context.Context, s domain.RouteSettings) error {
	headers := make([]headerJSON, len(s.ResponseHeaders))
	for i, h := range s.ResponseHeaders {
		headers[i] = headerJSON(h)
	}
	raw, err := json.Marshal(headers)
	if err != nil {
		return err
	}
	_, err = facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO route_settings (application_id, redirect, response_headers,
		    basic_auth_enabled, basic_auth_username, basic_auth_password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE
		SET redirect = EXCLUDED.redirect, response_headers = EXCLUDED.response_headers,
		    basic_auth_enabled = EXCLUDED.basic_auth_enabled, basic_auth_username = EXCLUDED.basic_auth_username,
		    basic_auth_password_hash = EXCLUDED.basic_auth_password_hash, updated_at = now()`,
		s.ApplicationID, string(s.Redirect), string(raw),
		s.BasicAuth.Enabled, s.BasicAuth.Username, s.BasicAuth.PasswordHash)
	return err
}

func (Settings) Delete(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&settingsRecord{})
	return err
}
