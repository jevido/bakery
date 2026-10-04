package infra

import (
	"context"
	"errors"
	"time"

	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

type scheduledBackupRecord struct {
	ID          uint64 `gorm:"primaryKey"`
	DatabaseID  uint64
	Enabled     bool
	Cron        string
	Retention   int
	S3StorageID *uint64 `gorm:"column:s3_storage_id"`
	EnabledAt   *time.Time
	orm.Timestamps
}

func (scheduledBackupRecord) TableName() string { return "scheduled_backups" }

func toScheduledBackupRecord(sb domain.ScheduledBackup) scheduledBackupRecord {
	return scheduledBackupRecord{
		ID: sb.ID, DatabaseID: sb.DatabaseID, Enabled: sb.Enabled, Cron: sb.Cron, Retention: sb.Retention,
		S3StorageID: nonZero(sb.S3StorageID), EnabledAt: nonZeroTime(sb.EnabledAt),
	}
}

func (r scheduledBackupRecord) toDomain() domain.ScheduledBackup {
	return domain.ScheduledBackup{
		ID: r.ID, DatabaseID: r.DatabaseID, Enabled: r.Enabled, Cron: r.Cron, Retention: r.Retention,
		S3StorageID: deref(r.S3StorageID), EnabledAt: derefTime(r.EnabledAt),
	}
}

func (s Store) CreateScheduledBackup(ctx context.Context, sb domain.ScheduledBackup) (domain.ScheduledBackup, error) {
	rec := toScheduledBackupRecord(sb)
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.ScheduledBackup{}, err
	}
	sb.ID = rec.ID
	return sb, nil
}

// ScheduledBackup returns the Scheduled backup; found is false when there is
// none.
func (s Store) ScheduledBackup(ctx context.Context, id uint64) (domain.ScheduledBackup, bool, error) {
	var rec scheduledBackupRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.ScheduledBackup{}, false, nil
		}
		return domain.ScheduledBackup{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Store) scheduledBackups(ctx context.Context, column string, value any) ([]domain.ScheduledBackup, error) {
	var recs []scheduledBackupRecord
	if err := s.query(ctx).Where(column, value).Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.ScheduledBackup, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// ScheduledBackups lists the Database's Scheduled backups, oldest first.
func (s Store) ScheduledBackups(ctx context.Context, databaseID uint64) ([]domain.ScheduledBackup, error) {
	return s.scheduledBackups(ctx, "database_id", databaseID)
}

// EnabledScheduledBackups lists every Scheduled backup that is on.
func (s Store) EnabledScheduledBackups(ctx context.Context) ([]domain.ScheduledBackup, error) {
	return s.scheduledBackups(ctx, "enabled", true)
}

func (s Store) SaveScheduledBackup(ctx context.Context, sb domain.ScheduledBackup) error {
	rec := toScheduledBackupRecord(sb)
	_, err := s.query(ctx).Model(&scheduledBackupRecord{}).Where("id", sb.ID).Update(map[string]any{
		"enabled": rec.Enabled, "cron": rec.Cron, "retention": rec.Retention,
		"s3_storage_id": rec.S3StorageID, "enabled_at": rec.EnabledAt,
	})
	return err
}

func (s Store) DeleteScheduledBackup(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&scheduledBackupRecord{})
	return err
}

// DeleteScheduledBackups removes every Scheduled backup of the Database.
func (s Store) DeleteScheduledBackups(ctx context.Context, databaseID uint64) error {
	_, err := s.query(ctx).Where("database_id", databaseID).Delete(&scheduledBackupRecord{})
	return err
}

// S3StorageInUse reports whether a Scheduled backup names the S3 storage.
func (s Store) S3StorageInUse(ctx context.Context, id uint64) (bool, error) {
	n, err := s.query(ctx).Model(&scheduledBackupRecord{}).Where("s3_storage_id", id).Count()
	return n > 0, err
}
