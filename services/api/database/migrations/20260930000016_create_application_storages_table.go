package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000016CreateApplicationStoragesTable holds the Persistent
// storages of each Application. The volumes themselves are Podman's and
// are named after the Application id and the storage name.
type M20260930000016CreateApplicationStoragesTable struct{}

func (r *M20260930000016CreateApplicationStoragesTable) Signature() string {
	return "20260930000016_create_application_storages_table"
}

func (r *M20260930000016CreateApplicationStoragesTable) Up() error {
	return facades.Schema().Create("application_storages", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.String("name")
		t.String("mount_path")
		t.Integer("position")
		t.TimestampsTz()
		t.Foreign("application_id").References("id").On("applications").CascadeOnDelete()
		t.Unique("application_id", "name")
	})
}

func (r *M20260930000016CreateApplicationStoragesTable) Down() error {
	return facades.Schema().DropIfExists("application_storages")
}
