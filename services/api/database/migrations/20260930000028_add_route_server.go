package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000028AddRouteServer records the Server a Route is served on (a
// Server id of the servers context, no foreign key); 0 is the Local server,
// where every existing Route is.
type M20260930000028AddRouteServer struct{}

func (r *M20260930000028AddRouteServer) Signature() string {
	return "20260930000028_add_route_server"
}

func (r *M20260930000028AddRouteServer) Up() error {
	return facades.Schema().Table("routes", func(t schema.Blueprint) {
		t.UnsignedBigInteger("server_id").Default(0)
	})
}

func (r *M20260930000028AddRouteServer) Down() error {
	return facades.Schema().DropColumns("routes", []string{"server_id"})
}
