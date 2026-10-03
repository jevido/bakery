package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000041RenameEngineAndDatabaseBackups gives the databases
// context Coolify's words: a Database's engine is its type, and a Backup is
// a Backup execution. The files on disk and in S3 keep their names.
type M20260930000041RenameEngineAndDatabaseBackups struct{}

func (r *M20260930000041RenameEngineAndDatabaseBackups) Signature() string {
	return "20260930000041_rename_engine_and_database_backups"
}

func (r *M20260930000041RenameEngineAndDatabaseBackups) Up() error {
	if err := facades.Schema().Sql(`ALTER TABLE databases RENAME COLUMN engine TO type`); err != nil {
		return err
	}
	return renameTables([][2]string{{"database_backups", "backup_executions"}})
}

func (r *M20260930000041RenameEngineAndDatabaseBackups) Down() error {
	if err := renameTables([][2]string{{"backup_executions", "database_backups"}}); err != nil {
		return err
	}
	return facades.Schema().Sql(`ALTER TABLE databases RENAME COLUMN type TO engine`)
}
