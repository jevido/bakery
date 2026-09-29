package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260929000001CreateUsersTable holds the Owner (identity). The table is
// called users so teams can add rows later without a rename.
type M20260929000001CreateUsersTable struct{}

func (r *M20260929000001CreateUsersTable) Signature() string {
	return "20260929000001_create_users_table"
}

func (r *M20260929000001CreateUsersTable) Up() error {
	if facades.Schema().HasTable("users") {
		return nil
	}
	return facades.Schema().Create("users", func(table schema.Blueprint) {
		table.ID()
		table.String("name")
		table.String("email")
		table.String("password")
		table.TimestampsTz()
		table.Unique("email")
	})
}

func (r *M20260929000001CreateUsersTable) Down() error {
	return facades.Schema().DropIfExists("users")
}
