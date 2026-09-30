package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000021CreateDatabaseBackupsTable holds every Backup of a
// Database and its outcome.
type M20260930000021CreateDatabaseBackupsTable struct{}

func (r *M20260930000021CreateDatabaseBackupsTable) Signature() string {
	return "20260930000021_create_database_backups_table"
}

func (r *M20260930000021CreateDatabaseBackupsTable) Up() error {
	return facades.Schema().Create("database_backups", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("database_id")
		t.String("status", 20)
		t.String("trigger", 20)
		t.String("file_name", 100)
		t.BigInteger("size_bytes").Default(0)
		t.Boolean("local").Default(false)
		t.Boolean("s3").Default(false)
		t.UnsignedBigInteger("s3_storage_id").Nullable()
		t.Text("error").Default("")
		t.TimestampTz("started_at")
		t.TimestampTz("finished_at").Nullable()
		t.TimestampsTz()
		t.Index("database_id", "started_at")
	})
}

func (r *M20260930000021CreateDatabaseBackupsTable) Down() error {
	return facades.Schema().DropIfExists("database_backups")
}
