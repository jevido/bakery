package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000024CreateServiceVariablesTable holds Service variables; the
// value is encrypted with the application key.
type M20260930000024CreateServiceVariablesTable struct{}

func (r *M20260930000024CreateServiceVariablesTable) Signature() string {
	return "20260930000024_create_service_variables_table"
}

func (r *M20260930000024CreateServiceVariablesTable) Up() error {
	return facades.Schema().Create("service_variables", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("service_id")
		t.String("name", 200)
		t.Text("value_encrypted")
		t.String("magic", 20).Default("")
		t.Text("default_value").Default("")
		t.Boolean("has_default").Default(false)
		t.Integer("position")
		t.TimestampsTz()
		t.Unique("service_id", "name")
	})
}

func (r *M20260930000024CreateServiceVariablesTable) Down() error {
	return facades.Schema().DropIfExists("service_variables")
}
