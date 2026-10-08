package infra

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type webhookRecord struct {
	ID              uint64 `gorm:"primaryKey"`
	ApplicationID   uint64
	SecretEncrypted string
	AutoDeploy      bool
	Previews        bool
	// GitHostTokenEncrypted is empty when there is no Git host token.
	GitHostTokenEncrypted string
	Provider              *string
	RepositoryAPI         *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (webhookRecord) TableName() string { return "webhooks" }

// Webhooks stores Webhooks with the secret encrypted by APP_KEY.
type Webhooks struct{}

func (Webhooks) ByApplication(ctx context.Context, applicationID uint64) (domain.Webhook, bool, error) {
	var recs []webhookRecord
	if err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Limit(1).Find(&recs); err != nil {
		return domain.Webhook{}, false, err
	}
	if len(recs) == 0 {
		return domain.Webhook{}, false, nil
	}
	secret, err := facades.Crypt().DecryptString(recs[0].SecretEncrypted)
	if err != nil {
		return domain.Webhook{}, false, errors.New("cannot decrypt the webhook secret (was APP_KEY changed?)")
	}
	var token string
	if recs[0].GitHostTokenEncrypted != "" {
		if token, err = facades.Crypt().DecryptString(recs[0].GitHostTokenEncrypted); err != nil {
			return domain.Webhook{}, false, errors.New("cannot decrypt the git host token (was APP_KEY changed?)")
		}
	}
	hook := domain.Webhook{ApplicationID: applicationID, Secret: secret, AutoDeploy: recs[0].AutoDeploy, Previews: recs[0].Previews, GitHostToken: token}
	if recs[0].Provider != nil {
		hook.Provider = domain.Provider(*recs[0].Provider)
	}
	if recs[0].RepositoryAPI != nil {
		hook.RepositoryAPI = *recs[0].RepositoryAPI
	}
	return hook, true, nil
}

func (Webhooks) Save(ctx context.Context, w domain.Webhook) error {
	enc, err := facades.Crypt().EncryptString(w.Secret)
	if err != nil {
		return err
	}
	var token string
	if w.GitHostToken != "" {
		if token, err = facades.Crypt().EncryptString(w.GitHostToken); err != nil {
			return err
		}
	}
	_, err = facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO webhooks (application_id, secret_encrypted, auto_deploy, previews, git_host_token_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE
		SET secret_encrypted = EXCLUDED.secret_encrypted, auto_deploy = EXCLUDED.auto_deploy, previews = EXCLUDED.previews,
		    git_host_token_encrypted = EXCLUDED.git_host_token_encrypted, updated_at = now()`,
		w.ApplicationID, enc, w.AutoDeploy, w.Previews, token)
	return err
}

func (Webhooks) RememberRepository(ctx context.Context, applicationID uint64, p domain.Provider, api string) error {
	var a *string
	if api != "" {
		a = &api
	}
	_, err := facades.Orm().WithContext(ctx).Query().Exec(`UPDATE webhooks SET provider = ?, repository_api = ?, updated_at = now() WHERE application_id = ?`, string(p), a, applicationID)
	return err
}

func (Webhooks) DeleteForApplication(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&webhookRecord{})
	return err
}
