package infra

import (
	"context"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

type deliveryRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ChannelID     uint64
	EventKind     string
	Title         string
	Body          string
	Link          string
	HappenedAt    time.Time
	Status        string
	Attempts      int
	LastError     string
	NextAttemptAt *time.Time
	SentAt        *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (deliveryRecord) TableName() string { return "notification_deliveries" }

func optional(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	t = t.UTC()
	return &t
}

func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

func toDeliveryRecord(d domain.Delivery) deliveryRecord {
	n := d.Notification
	return deliveryRecord{
		ID: d.ID, ChannelID: d.ChannelID, EventKind: string(n.Kind), Title: n.Title, Body: n.Body, Link: n.Link,
		HappenedAt: n.At.UTC(), Status: string(d.Status), Attempts: d.Attempts, LastError: d.LastError,
		NextAttemptAt: optional(d.NextAttemptAt), SentAt: optional(d.SentAt), CreatedAt: d.CreatedAt.UTC(), UpdatedAt: time.Now().UTC(),
	}
}

func (r deliveryRecord) toDomain() domain.Delivery {
	return domain.Delivery{
		ID: r.ID, ChannelID: r.ChannelID,
		Notification: domain.Notification{Kind: domain.EventKind(r.EventKind), Title: r.Title, Body: r.Body, Link: r.Link, At: r.HappenedAt.UTC()},
		Status:       domain.DeliveryStatus(r.Status), Attempts: r.Attempts, LastError: r.LastError,
		NextAttemptAt: deref(r.NextAttemptAt), SentAt: deref(r.SentAt), CreatedAt: r.CreatedAt.UTC(),
	}
}

// CreateDeliveries stores new Deliveries and returns them with their ids.
func (s Store) CreateDeliveries(ctx context.Context, ds []domain.Delivery) ([]domain.Delivery, error) {
	if len(ds) == 0 {
		return nil, nil
	}
	recs := make([]deliveryRecord, len(ds))
	for i, d := range ds {
		recs[i] = toDeliveryRecord(d)
	}
	if err := s.query(ctx).Create(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Delivery, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// ClaimDue returns up to limit pending Deliveries due at now, and pushes
// their next attempt back by lease, so another dispatcher (or this one
// after a crash mid-send) only tries them again once that has passed.
func (s Store) ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	var recs []deliveryRecord
	err := s.query(ctx).Raw(`
		UPDATE notification_deliveries SET next_attempt_at = ?, updated_at = now()
		WHERE id IN (
			SELECT id FROM notification_deliveries
			WHERE status = 'pending' AND next_attempt_at <= ?
			ORDER BY next_attempt_at, id FOR UPDATE SKIP LOCKED LIMIT ?
		)
		RETURNING *`, now.Add(lease).UTC(), now.UTC(), limit).Scan(&recs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Delivery, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// SaveDelivery writes the outcome of an attempt, then keeps only the
// channel's newest Deliveries.
func (s Store) SaveDelivery(ctx context.Context, d domain.Delivery) error {
	rec := toDeliveryRecord(d)
	if _, err := s.query(ctx).Model(&deliveryRecord{}).Where("id", d.ID).Update(map[string]any{
		"status": rec.Status, "attempts": rec.Attempts, "last_error": rec.LastError,
		"next_attempt_at": rec.NextAttemptAt, "sent_at": rec.SentAt, "updated_at": rec.UpdatedAt,
	}); err != nil {
		return err
	}
	_, err := s.query(ctx).Exec(`
		DELETE FROM notification_deliveries WHERE channel_id = ? AND status <> 'pending' AND id < (
			SELECT min(id) FROM (
				SELECT id FROM notification_deliveries WHERE channel_id = ? ORDER BY id DESC LIMIT ?
			) newest
		)`, d.ChannelID, d.ChannelID, domain.KeptDeliveries)
	return err
}

// Deliveries returns the channel's newest Deliveries, newest first.
func (s Store) Deliveries(ctx context.Context, channelID uint64, limit int) ([]domain.Delivery, error) {
	var recs []deliveryRecord
	if err := s.query(ctx).Where("channel_id", channelID).Order("id desc").Limit(limit).Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Delivery, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}
