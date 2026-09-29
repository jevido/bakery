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
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
	return domain.Webhook{ApplicationID: applicationID, Secret: secret, AutoDeploy: recs[0].AutoDeploy}, true, nil
}

func (Webhooks) Save(ctx context.Context, w domain.Webhook) error {
	enc, err := facades.Crypt().EncryptString(w.Secret)
	if err != nil {
		return err
	}
	_, err = facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO webhooks (application_id, secret_encrypted, auto_deploy, created_at, updated_at)
		VALUES (?, ?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE
		SET secret_encrypted = EXCLUDED.secret_encrypted, auto_deploy = EXCLUDED.auto_deploy, updated_at = now()`,
		w.ApplicationID, enc, w.AutoDeploy)
	return err
}

func (Webhooks) DeleteForApplication(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&webhookRecord{})
	return err
}
