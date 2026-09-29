package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000005CreateWebhooksTable holds deployments' Webhooks, one per
// Application. No foreign key to applications: the ApplicationDeleted event
// removes the row.
type M20260930000005CreateWebhooksTable struct{}

func (r *M20260930000005CreateWebhooksTable) Signature() string {
	return "20260930000005_create_webhooks_table"
}

func (r *M20260930000005CreateWebhooksTable) Up() error {
	return facades.Schema().Create("webhooks", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.Text("secret_encrypted")
		t.Boolean("auto_deploy").Default(true)
		t.TimestampsTz()
		t.Unique("application_id")
	})
}

func (r *M20260930000005CreateWebhooksTable) Down() error {
	return facades.Schema().DropIfExists("webhooks")
}
