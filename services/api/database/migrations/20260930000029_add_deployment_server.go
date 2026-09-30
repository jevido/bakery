package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000029AddDeploymentServer records the Server a Deployment ran on
// (a Server id of the servers context, no foreign key); 0 is the Local
// server, where every existing Deployment ran.
type M20260930000029AddDeploymentServer struct{}

func (r *M20260930000029AddDeploymentServer) Signature() string {
	return "20260930000029_add_deployment_server"
}

func (r *M20260930000029AddDeploymentServer) Up() error {
	return facades.Schema().Table("deployments", func(t schema.Blueprint) {
		t.UnsignedBigInteger("server_id").Default(0)
	})
}

func (r *M20260930000029AddDeploymentServer) Down() error {
	return facades.Schema().DropColumns("deployments", []string{"server_id"})
}
