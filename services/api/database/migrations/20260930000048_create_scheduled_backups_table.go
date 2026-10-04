package migrations

// M20260930000048CreateScheduledBackupsTable makes Scheduled backup its own
// table, many per Database, as Coolify's scheduled database backups. Every
// Database of a type with backups gets one row from its backup_* columns
// (switched off ones too, so their Backup executions keep an owner), every
// Backup execution points at it, and the columns go.
type M20260930000048CreateScheduledBackupsTable struct{}

func (r *M20260930000048CreateScheduledBackupsTable) Signature() string {
	return "20260930000048_create_scheduled_backups_table"
}

func (r *M20260930000048CreateScheduledBackupsTable) Up() error {
	return sqls(
		`CREATE TABLE scheduled_backups (
			id bigserial PRIMARY KEY,
			database_id bigint NOT NULL,
			enabled boolean NOT NULL DEFAULT false,
			cron varchar(100) NOT NULL,
			retention integer NOT NULL,
			s3_storage_id bigint NULL,
			enabled_at timestamptz NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
		`CREATE INDEX scheduled_backups_database_id_index ON scheduled_backups (database_id)`,
		`CREATE INDEX scheduled_backups_enabled_index ON scheduled_backups (enabled)`,
		`INSERT INTO scheduled_backups (database_id, enabled, cron, retention, s3_storage_id, enabled_at, created_at, updated_at)
			SELECT id, backup_enabled, backup_cron, backup_retention, backup_s3_storage_id, backup_enabled_at, now(), now()
			FROM databases
			WHERE type IN ('postgresql', 'mysql', 'mariadb', 'mongodb')
				OR id IN (SELECT database_id FROM backup_executions)
			ORDER BY id`,
		`ALTER TABLE backup_executions ADD COLUMN scheduled_backup_id bigint`,
		`UPDATE backup_executions e SET scheduled_backup_id = s.id FROM scheduled_backups s WHERE s.database_id = e.database_id`,
		// A Backup execution whose Database is gone has no page to show on.
		`DELETE FROM backup_executions WHERE scheduled_backup_id IS NULL`,
		`ALTER TABLE backup_executions ALTER COLUMN scheduled_backup_id SET NOT NULL`,
		`CREATE INDEX backup_executions_scheduled_backup_id_index ON backup_executions (scheduled_backup_id)`,
		`ALTER TABLE databases
			DROP COLUMN backup_enabled, DROP COLUMN backup_cron, DROP COLUMN backup_retention,
			DROP COLUMN backup_s3_storage_id, DROP COLUMN backup_enabled_at`,
	)
}

// Down puts each Database's oldest Scheduled backup back in its columns;
// the others are lost, and their Backup executions stay with the Database.
func (r *M20260930000048CreateScheduledBackupsTable) Down() error {
	return sqls(
		`ALTER TABLE databases
			ADD COLUMN backup_enabled boolean NOT NULL DEFAULT false,
			ADD COLUMN backup_cron varchar(100) NOT NULL DEFAULT '0 3 * * *',
			ADD COLUMN backup_retention integer NOT NULL DEFAULT 7,
			ADD COLUMN backup_s3_storage_id bigint NULL,
			ADD COLUMN backup_enabled_at timestamptz NULL`,
		`UPDATE databases d SET backup_enabled = s.enabled, backup_cron = s.cron, backup_retention = s.retention,
				backup_s3_storage_id = s.s3_storage_id, backup_enabled_at = s.enabled_at
			FROM (SELECT DISTINCT ON (database_id) * FROM scheduled_backups ORDER BY database_id, id) s
			WHERE s.database_id = d.id`,
		`ALTER TABLE backup_executions DROP COLUMN scheduled_backup_id`,
		`DROP TABLE scheduled_backups`,
	)
}
