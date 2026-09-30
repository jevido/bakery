package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000023CreateServiceComponentsTable holds each Service's
// Components; domains is a JSON array, primary first.
type M20260930000023CreateServiceComponentsTable struct{}

func (r *M20260930000023CreateServiceComponentsTable) Signature() string {
	return "20260930000023_create_service_components_table"
}

func (r *M20260930000023CreateServiceComponentsTable) Up() error {
	return facades.Schema().Create("service_components", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("service_id")
		t.String("name", 63)
		t.Text("image")
		t.Boolean("public").Default(false)
		t.Integer("port").Default(0)
		t.Text("domains").Default("[]")
		t.Integer("position")
		t.TimestampsTz()
		t.Unique("service_id", "name")
	})
}

func (r *M20260930000023CreateServiceComponentsTable) Down() error {
	return facades.Schema().DropIfExists("service_components")
}
