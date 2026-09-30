package infra

import (
	"context"
	"errors"
	"time"

	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

type backupRecord struct {
	ID          uint64 `gorm:"primaryKey"`
	DatabaseID  uint64
	Status      string
	Trigger     string
	FileName    string
	SizeBytes   int64
	Local       bool
	S3          bool    `gorm:"column:s3"`
	S3StorageID *uint64 `gorm:"column:s3_storage_id"`
	Error       string
	StartedAt   time.Time
	FinishedAt  *time.Time
	orm.Timestamps
}

func (backupRecord) TableName() string { return "database_backups" }

func toBackupRecord(b domain.Backup) backupRecord {
	return backupRecord{
		ID: b.ID, DatabaseID: b.DatabaseID, Status: string(b.Status), Trigger: string(b.Trigger),
		FileName: b.FileName, SizeBytes: b.SizeBytes, Local: b.Local, S3: b.S3,
		S3StorageID: nonZero(b.S3StorageID), Error: b.Error,
		StartedAt: b.StartedAt.UTC(), FinishedAt: nonZeroTime(b.FinishedAt),
	}
}

func (r backupRecord) toDomain() domain.Backup {
	return domain.Backup{
		ID: r.ID, DatabaseID: r.DatabaseID, Status: domain.BackupStatus(r.Status), Trigger: domain.BackupTrigger(r.Trigger),
		FileName: r.FileName, SizeBytes: r.SizeBytes, Local: r.Local, S3: r.S3,
		S3StorageID: deref(r.S3StorageID), Error: r.Error,
		StartedAt: r.StartedAt.UTC(), FinishedAt: derefTime(r.FinishedAt),
	}
}

func (s Store) CreateBackup(ctx context.Context, b domain.Backup) (domain.Backup, error) {
	rec := toBackupRecord(b)
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Backup{}, err
	}
	b.ID = rec.ID
	return b, nil
}

func (s Store) SaveBackup(ctx context.Context, b domain.Backup) error {
	rec := toBackupRecord(b)
	_, err := s.query(ctx).Model(&backupRecord{}).Where("id", b.ID).Update(map[string]any{
		"status": rec.Status, "size_bytes": rec.SizeBytes, "local": rec.Local, "s3": rec.S3,
		"s3_storage_id": rec.S3StorageID, "error": rec.Error, "finished_at": rec.FinishedAt,
	})
	return err
}

// Backup returns the Backup; found is false when there is none.
func (s Store) Backup(ctx context.Context, id uint64) (domain.Backup, bool, error) {
	var rec backupRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Backup{}, false, nil
		}
		return domain.Backup{}, false, err
	}
	return rec.toDomain(), true, nil
}

// Backups lists the Database's Backups, newest first.
func (s Store) Backups(ctx context.Context, databaseID uint64) ([]domain.Backup, error) {
	var recs []backupRecord
	if err := s.query(ctx).Where("database_id", databaseID).Order("started_at desc").Order("id desc").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Backup, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// LastScheduledStart is when the Database's newest scheduled Backup
// started; the zero time without one.
func (s Store) LastScheduledStart(ctx context.Context, databaseID uint64) (time.Time, error) {
	var rec backupRecord
	err := s.query(ctx).Where("database_id", databaseID).Where("trigger", string(domain.TriggerScheduled)).Order("started_at desc").FirstOrFail(&rec)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return rec.StartedAt.UTC(), nil
}

func (s Store) DeleteBackup(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&backupRecord{})
	return err
}

// DeleteBackups removes the rows of every Backup of the Database.
func (s Store) DeleteBackups(ctx context.Context, databaseID uint64) error {
	_, err := s.query(ctx).Where("database_id", databaseID).Delete(&backupRecord{})
	return err
}

// FailRunningBackups marks every running Backup failed with reason.
func (s Store) FailRunningBackups(ctx context.Context, reason string, at time.Time) (int64, error) {
	res, err := s.query(ctx).Model(&backupRecord{}).Where("status", string(domain.BackupRunning)).Update(map[string]any{
		"status": string(domain.BackupFailed), "error": reason, "finished_at": at.UTC(),
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected, nil
}

type s3StorageRecord struct {
	ID                 uint64 `gorm:"primaryKey"`
	Name               string
	Endpoint           string
	Region             string
	Bucket             string
	Prefix             string
	AccessKey          string
	SecretKeyEncrypted string
	orm.Timestamps
}

func (s3StorageRecord) TableName() string { return "s3_storages" }

func (r s3StorageRecord) toDomain() (domain.S3Storage, error) {
	secret, err := decrypt(r.SecretKeyEncrypted)
	return domain.S3Storage{
		ID: r.ID, Name: r.Name, Endpoint: r.Endpoint, Region: r.Region, Bucket: r.Bucket,
		Prefix: r.Prefix, AccessKey: r.AccessKey, SecretKey: secret,
	}, err
}

func toS3StorageRecord(st domain.S3Storage) (s3StorageRecord, error) {
	secret, err := encrypt(st.SecretKey)
	return s3StorageRecord{
		ID: st.ID, Name: st.Name, Endpoint: st.Endpoint, Region: st.Region, Bucket: st.Bucket,
		Prefix: st.Prefix, AccessKey: st.AccessKey, SecretKeyEncrypted: secret,
	}, err
}

// S3Storages lists every S3 storage by name.
func (s Store) S3Storages(ctx context.Context) ([]domain.S3Storage, error) {
	var recs []s3StorageRecord
	if err := s.query(ctx).Order("name").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.S3Storage, len(recs))
	for i, r := range recs {
		st, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = st
	}
	return out, nil
}

// S3Storage returns the S3 storage; found is false when there is none.
func (s Store) S3Storage(ctx context.Context, id uint64) (domain.S3Storage, bool, error) {
	var rec s3StorageRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.S3Storage{}, false, nil
		}
		return domain.S3Storage{}, false, err
	}
	st, err := rec.toDomain()
	return st, err == nil, err
}

// S3StorageNameTaken reports whether an S3 storage other than exceptID has
// the name.
func (s Store) S3StorageNameTaken(ctx context.Context, name string, exceptID uint64) (bool, error) {
	n, err := s.query(ctx).Model(&s3StorageRecord{}).Where("name", name).Where("id <> ?", exceptID).Count()
	return n > 0, err
}

func (s Store) CreateS3Storage(ctx context.Context, st domain.S3Storage) (domain.S3Storage, error) {
	rec, err := toS3StorageRecord(st)
	if err != nil {
		return domain.S3Storage{}, err
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.S3Storage{}, err
	}
	st.ID = rec.ID
	return st, nil
}

func (s Store) SaveS3Storage(ctx context.Context, st domain.S3Storage) error {
	rec, err := toS3StorageRecord(st)
	if err != nil {
		return err
	}
	_, err = s.query(ctx).Model(&s3StorageRecord{}).Where("id", st.ID).Update(map[string]any{
		"name": rec.Name, "endpoint": rec.Endpoint, "region": rec.Region, "bucket": rec.Bucket,
		"prefix": rec.Prefix, "access_key": rec.AccessKey, "secret_key_encrypted": rec.SecretKeyEncrypted,
	})
	return err
}

func (s Store) DeleteS3Storage(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&s3StorageRecord{})
	return err
}
