package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000027AddApplicationServer holds an Application's Target server,
// a Server id of the servers context (no foreign key across contexts). NULL
// is the Local server, so existing Applications stay where they are.
type M20260930000027AddApplicationServer struct{}

func (r *M20260930000027AddApplicationServer) Signature() string {
	return "20260930000027_add_application_server"
}

func (r *M20260930000027AddApplicationServer) Up() error {
	return facades.Schema().Table("applications", func(t schema.Blueprint) {
		t.UnsignedBigInteger("server_id").Nullable()
		t.Index("server_id")
	})
}

func (r *M20260930000027AddApplicationServer) Down() error {
	return facades.Schema().DropColumns("applications", []string{"server_id"})
}
