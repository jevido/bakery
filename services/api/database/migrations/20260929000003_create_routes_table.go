package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260929000003CreateRoutesTable holds the routing context's Routes. No
// foreign key to applications: routing learns of deletions through the
// ApplicationDeleted event, not another context's table.
type M20260929000003CreateRoutesTable struct{}

func (r *M20260929000003CreateRoutesTable) Signature() string {
	return "20260929000003_create_routes_table"
}

func (r *M20260929000003CreateRoutesTable) Up() error {
	return facades.Schema().Create("routes", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.String("domain")
		t.String("container_name")
		t.Integer("container_port")
		t.TimestampsTz()
		t.Unique("application_id")
		t.Unique("domain")
	})
}

func (r *M20260929000003CreateRoutesTable) Down() error {
	return facades.Schema().DropIfExists("routes")
}
