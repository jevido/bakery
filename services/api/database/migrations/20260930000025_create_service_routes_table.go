package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000025CreateServiceRoutesTable holds routing's Service routes:
// a Public Component's Domains (a JSON array) → Container and port.
type M20260930000025CreateServiceRoutesTable struct{}

func (r *M20260930000025CreateServiceRoutesTable) Signature() string {
	return "20260930000025_create_service_routes_table"
}

func (r *M20260930000025CreateServiceRoutesTable) Up() error {
	return facades.Schema().Create("service_routes", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("service_id")
		t.String("component", 63)
		t.Text("domains").Default("[]")
		t.String("container_name", 255)
		t.Integer("container_port")
		t.TimestampsTz()
		t.Unique("service_id", "component")
	})
}

func (r *M20260930000025CreateServiceRoutesTable) Down() error {
	return facades.Schema().DropIfExists("service_routes")
}
