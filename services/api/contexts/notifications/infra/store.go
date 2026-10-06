// Package infra is the notifications context's persistence and its senders.
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

type channelRecord struct {
	ID                uint64 `gorm:"primaryKey"`
	Name              string
	Kind              string
	SettingsEncrypted string
	EventKinds        string
	Enabled           bool
	orm.Timestamps
}

func (channelRecord) TableName() string { return "notification_channels" }

func toRecord(c domain.Channel) (channelRecord, error) {
	raw, err := json.Marshal(c.Settings)
	if err != nil {
		return channelRecord{}, err
	}
	enc, err := facades.Crypt().EncryptString(string(raw))
	if err != nil {
		return channelRecord{}, err
	}
	kinds := make([]string, len(c.EventKinds))
	for i, k := range c.EventKinds {
		kinds[i] = string(k)
	}
	return channelRecord{ID: c.ID, Name: c.Name, Kind: string(c.Kind), SettingsEncrypted: enc, EventKinds: strings.Join(kinds, ","), Enabled: c.Enabled}, nil
}

func (r channelRecord) toDomain() (domain.Channel, error) {
	raw, err := facades.Crypt().DecryptString(r.SettingsEncrypted)
	if err != nil {
		return domain.Channel{}, err
	}
	c := domain.Channel{ID: r.ID, Name: r.Name, Kind: domain.Kind(r.Kind), Enabled: r.Enabled, CreatedAt: createdAt(r.Timestamps)}
	if err := json.Unmarshal([]byte(raw), &c.Settings); err != nil {
		return domain.Channel{}, err
	}
	for _, k := range strings.Split(r.EventKinds, ",") {
		if k != "" {
			c.EventKinds = append(c.EventKinds, domain.EventKind(k))
		}
	}
	return c, nil
}

func createdAt(t orm.Timestamps) time.Time {
	if t.CreatedAt == nil {
		return time.Time{}
	}
	return t.CreatedAt.StdTime().UTC()
}

// Store keeps Notification channels, their settings encrypted with the
// application key, and their Deliveries.
type Store struct{}

func (Store) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Store) Channels(ctx context.Context) ([]domain.Channel, error) {
	var recs []channelRecord
	if err := s.query(ctx).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Channel, len(recs))
	for i, r := range recs {
		c, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// Channel returns the channel; found is false when there is none.
func (s Store) Channel(ctx context.Context, id uint64) (domain.Channel, bool, error) {
	var rec channelRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Channel{}, false, nil
		}
		return domain.Channel{}, false, err
	}
	c, err := rec.toDomain()
	return c, err == nil, err
}

func (s Store) CreateChannel(ctx context.Context, c domain.Channel) (domain.Channel, error) {
	rec, err := toRecord(c)
	if err != nil {
		return domain.Channel{}, err
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Channel{}, err
	}
	c.ID = rec.ID
	c.CreatedAt = createdAt(rec.Timestamps)
	return c, nil
}

func (s Store) SaveChannel(ctx context.Context, c domain.Channel) error {
	rec, err := toRecord(c)
	if err != nil {
		return err
	}
	_, err = s.query(ctx).Model(&channelRecord{}).Where("id", c.ID).Update(map[string]any{
		"name": rec.Name, "settings_encrypted": rec.SettingsEncrypted, "event_kinds": rec.EventKinds, "enabled": rec.Enabled,
	})
	return err
}

func (s Store) DeleteChannel(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&channelRecord{})
	return err
}

// ChannelNameTaken reports whether a channel other than exceptID has the
// name (ignoring case).
func (s Store) ChannelNameTaken(ctx context.Context, name string, exceptID uint64) (bool, error) {
	n, err := s.query(ctx).Model(&channelRecord{}).Where("lower(name) = lower(?)", name).Where("id <> ?", exceptID).Count()
	return n > 0, err
}
