// Package infra is the databases context's persistence and Podman runtime.
package infra

import (
	"context"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

type databaseRecord struct {
	ID                    uint64 `gorm:"primaryKey"`
	EnvironmentID         uint64
	ProjectID             uint64
	Name                  string
	Slug                  string
	Engine                string
	Version               string
	Username              string
	PasswordEncrypted     string
	RootPasswordEncrypted string
	DatabaseName          string
	PublicPort            *int
	MemoryMB              int     `gorm:"column:memory_mb"`
	CPUs                  float64 `gorm:"column:cpus"`
	DesiredState          string
	BackupEnabled         bool
	BackupCron            string
	BackupRetention       int
	BackupS3StorageID     *uint64 `gorm:"column:backup_s3_storage_id"`
	BackupEnabledAt       *time.Time
	orm.Timestamps
}

func (databaseRecord) TableName() string { return "databases" }

func encrypt(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	return facades.Crypt().EncryptString(s)
}

func decrypt(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	return facades.Crypt().DecryptString(s)
}

func toRecord(d domain.Database) (databaseRecord, error) {
	password, err := encrypt(d.Credentials.Password)
	if err != nil {
		return databaseRecord{}, err
	}
	root, err := encrypt(d.Credentials.RootPassword)
	if err != nil {
		return databaseRecord{}, err
	}
	rec := databaseRecord{
		ID: d.ID, EnvironmentID: d.EnvironmentID, ProjectID: d.ProjectID,
		Name: d.Name, Slug: d.Slug, Engine: string(d.Engine), Version: d.Version,
		Username: d.Credentials.Username, PasswordEncrypted: password, RootPasswordEncrypted: root,
		DatabaseName: d.Credentials.DatabaseName,
		MemoryMB:     d.ResourceLimits.MemoryMB, CPUs: d.ResourceLimits.CPUs,
		DesiredState:  string(d.DesiredState),
		BackupEnabled: d.BackupSchedule.Enabled, BackupCron: d.BackupSchedule.Cron,
		BackupRetention:   d.BackupSchedule.Retention,
		BackupS3StorageID: nonZero(d.BackupSchedule.S3StorageID), BackupEnabledAt: nonZeroTime(d.BackupSchedule.EnabledAt),
	}
	if d.PublicPort != 0 {
		port := d.PublicPort
		rec.PublicPort = &port
	}
	return rec, nil
}

func (r databaseRecord) toDomain() (domain.Database, error) {
	password, err := decrypt(r.PasswordEncrypted)
	if err != nil {
		return domain.Database{}, err
	}
	root, err := decrypt(r.RootPasswordEncrypted)
	if err != nil {
		return domain.Database{}, err
	}
	d := domain.Database{
		ID: r.ID, EnvironmentID: r.EnvironmentID, ProjectID: r.ProjectID,
		Name: r.Name, Slug: r.Slug, Engine: domain.Engine(r.Engine), Version: r.Version,
		Credentials:    domain.Credentials{Username: r.Username, Password: password, RootPassword: root, DatabaseName: r.DatabaseName},
		ResourceLimits: domain.ResourceLimits{MemoryMB: r.MemoryMB, CPUs: r.CPUs},
		DesiredState:   domain.DesiredState(r.DesiredState),
		BackupSchedule: domain.BackupSchedule{
			Enabled: r.BackupEnabled, Cron: r.BackupCron, Retention: r.BackupRetention,
			S3StorageID: deref(r.BackupS3StorageID), EnabledAt: derefTime(r.BackupEnabledAt),
		},
	}
	if r.PublicPort != nil {
		d.PublicPort = *r.PublicPort
	}
	return d, nil
}

// Store keeps Databases; passwords are encrypted with the application key.
type Store struct{}

func (Store) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Store) Create(ctx context.Context, d domain.Database) (domain.Database, error) {
	rec, err := toRecord(d)
	if err != nil {
		return domain.Database{}, err
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Database{}, err
	}
	d.ID = rec.ID
	return d, nil
}

// Get returns the Database; found is false when there is none.
func (s Store) Get(ctx context.Context, id uint64) (domain.Database, bool, error) {
	var rec databaseRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Database{}, false, nil
		}
		return domain.Database{}, false, err
	}
	d, err := rec.toDomain()
	return d, err == nil, err
}

func (s Store) list(q contractsorm.Query) ([]domain.Database, error) {
	var recs []databaseRecord
	if err := q.Order("id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Database, len(recs))
	for i, r := range recs {
		d, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = d
	}
	return out, nil
}

// ForProject lists the Project's Databases, oldest first.
func (s Store) ForProject(ctx context.Context, projectID uint64) ([]domain.Database, error) {
	return s.list(s.query(ctx).Where("project_id", projectID))
}

// Wanted lists every Database whose desired state is running.
func (s Store) Wanted(ctx context.Context) ([]domain.Database, error) {
	return s.list(s.query(ctx).Where("desired_state", string(domain.Running)))
}

func (s Store) Update(ctx context.Context, d domain.Database) error {
	rec, err := toRecord(d)
	if err != nil {
		return err
	}
	_, err = s.query(ctx).Model(&databaseRecord{}).Where("id", d.ID).Update(map[string]any{
		"name": rec.Name, "version": rec.Version, "public_port": rec.PublicPort,
		"memory_mb": rec.MemoryMB, "cpus": rec.CPUs, "desired_state": rec.DesiredState,
		"backup_enabled": rec.BackupEnabled, "backup_cron": rec.BackupCron, "backup_retention": rec.BackupRetention,
		"backup_s3_storage_id": rec.BackupS3StorageID, "backup_enabled_at": rec.BackupEnabledAt,
	})
	return err
}

func (s Store) Delete(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&databaseRecord{})
	return err
}

func (s Store) SlugTaken(ctx context.Context, slug string) (bool, error) {
	n, err := s.query(ctx).Model(&databaseRecord{}).Where("slug", slug).Count()
	return n > 0, err
}

// PublicPortTaken reports whether a Database other than exceptID has the
// Public port.
func (s Store) PublicPortTaken(ctx context.Context, port int, exceptID uint64) (bool, error) {
	n, err := s.query(ctx).Model(&databaseRecord{}).Where("public_port", port).Where("id <> ?", exceptID).Count()
	return n > 0, err
}

func (s Store) CountForProject(ctx context.Context, projectID uint64) (int64, error) {
	return s.query(ctx).Model(&databaseRecord{}).Where("project_id", projectID).Count()
}

// ScheduledDatabases lists every Database whose Backup schedule is on.
func (s Store) ScheduledDatabases(ctx context.Context) ([]domain.Database, error) {
	return s.list(s.query(ctx).Where("backup_enabled", true))
}

// S3StorageInUse reports whether a Backup schedule names the S3 storage.
func (s Store) S3StorageInUse(ctx context.Context, id uint64) (bool, error) {
	n, err := s.query(ctx).Model(&databaseRecord{}).Where("backup_s3_storage_id", id).Count()
	return n > 0, err
}

func nonZero(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

func deref(id *uint64) uint64 {
	if id == nil {
		return 0
	}
	return *id
}

func nonZeroTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	t = t.UTC()
	return &t
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}
