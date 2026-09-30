package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000018CreateDatabasesTable holds the databases context's
// Databases. No foreign keys to environments or projects: databases only
// references them by id, and projects asks databases before deleting a
// Project.
type M20260930000018CreateDatabasesTable struct{}

func (r *M20260930000018CreateDatabasesTable) Signature() string {
	return "20260930000018_create_databases_table"
}

func (r *M20260930000018CreateDatabasesTable) Up() error {
	return facades.Schema().Create("databases", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("environment_id")
		t.UnsignedBigInteger("project_id")
		t.String("name", 100)
		t.String("slug", 40)
		t.String("engine", 20)
		t.String("version", 128)
		t.String("username", 100)
		t.Text("password_encrypted")
		t.Text("root_password_encrypted").Default("")
		t.String("database_name", 100)
		t.Integer("public_port").Nullable()
		t.Integer("memory_mb").Default(0)
		t.Float("cpus").Default(0)
		t.String("desired_state", 10).Default("running")
		t.TimestampsTz()
		t.Unique("slug")
		t.Unique("public_port")
		t.Index("project_id")
	})
}

func (r *M20260930000018CreateDatabasesTable) Down() error {
	return facades.Schema().DropIfExists("databases")
}
