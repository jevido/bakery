package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000026CreateServersTable holds the servers context's Servers.
// The Local server has an empty host, port 0 and an empty user, so the
// unique address never collides with a Remote server. validation is the
// latest Validation as JSON.
type M20260930000026CreateServersTable struct{}

func (r *M20260930000026CreateServersTable) Signature() string {
	return "20260930000026_create_servers_table"
}

func (r *M20260930000026CreateServersTable) Up() error {
	return facades.Schema().Create("servers", func(t schema.Blueprint) {
		t.ID()
		t.String("name", 63)
		t.String("kind", 16)
		t.String("host", 253).Default("")
		t.Integer("port").Default(0)
		t.String("user_name", 32).Default("")
		t.Text("public_key").Default("")
		t.Text("private_key_encrypted").Default("")
		t.Text("host_key").Default("")
		t.String("status", 16)
		t.Text("validation").Default("{}")
		t.TimestampTz("last_cleanup_at").Nullable()
		t.BigInteger("last_cleanup_reclaimed").Default(0)
		t.TimestampsTz()
		t.Unique("name")
		t.Unique("host", "port", "user_name")
	})
}

func (r *M20260930000026CreateServersTable) Down() error {
	return facades.Schema().DropIfExists("servers")
}
