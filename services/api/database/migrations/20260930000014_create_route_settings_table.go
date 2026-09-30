package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000014CreateRouteSettingsTable holds the routing context's Route
// settings, one row per Application. No foreign key to applications, like
// routes: routing learns of deletions through ApplicationDeleted.
type M20260930000014CreateRouteSettingsTable struct{}

func (r *M20260930000014CreateRouteSettingsTable) Signature() string {
	return "20260930000014_create_route_settings_table"
}

func (r *M20260930000014CreateRouteSettingsTable) Up() error {
	return facades.Schema().Create("route_settings", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.String("www_redirect").Default("off")
		t.TimestampsTz()
		t.Unique("application_id")
	})
}

func (r *M20260930000014CreateRouteSettingsTable) Down() error {
	return facades.Schema().DropIfExists("route_settings")
}
