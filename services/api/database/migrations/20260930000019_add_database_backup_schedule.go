package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000019AddDatabaseBackupSchedule adds each Database's Backup
// schedule, off for every existing Database.
type M20260930000019AddDatabaseBackupSchedule struct{}

func (r *M20260930000019AddDatabaseBackupSchedule) Signature() string {
	return "20260930000019_add_database_backup_schedule"
}

func (r *M20260930000019AddDatabaseBackupSchedule) Up() error {
	return facades.Schema().Table("databases", func(t schema.Blueprint) {
		t.Boolean("backup_enabled").Default(false)
		t.String("backup_cron", 100).Default("0 3 * * *")
		t.Integer("backup_retention").Default(7)
		t.UnsignedBigInteger("backup_s3_storage_id").Nullable()
		t.TimestampTz("backup_enabled_at").Nullable()
	})
}

func (r *M20260930000019AddDatabaseBackupSchedule) Down() error {
	return facades.Schema().DropColumns("databases", []string{"backup_enabled", "backup_cron", "backup_retention", "backup_s3_storage_id", "backup_enabled_at"})
}
