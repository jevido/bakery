package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000022CreateServicesTable holds Services: a Compose file run in
// an Environment. environment_id and project_id are projects' ids, without
// a foreign key, as for databases.
type M20260930000022CreateServicesTable struct{}

func (r *M20260930000022CreateServicesTable) Signature() string {
	return "20260930000022_create_services_table"
}

func (r *M20260930000022CreateServicesTable) Up() error {
	return facades.Schema().Create("services", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("environment_id")
		t.UnsignedBigInteger("project_id")
		t.String("name", 100)
		t.String("slug", 50)
		t.Text("compose_file")
		t.String("template_key", 100).Default("")
		t.String("desired_state", 20)
		t.Text("last_error").Default("")
		t.TimestampsTz()
		t.Unique("slug")
		t.Index("project_id")
	})
}

func (r *M20260930000022CreateServicesTable) Down() error {
	return facades.Schema().DropIfExists("services")
}
