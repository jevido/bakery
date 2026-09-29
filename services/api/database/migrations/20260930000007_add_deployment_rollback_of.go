package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000007AddDeploymentRollbackOf names the Deployment a Rollback
// starts the Image of.
type M20260930000007AddDeploymentRollbackOf struct{}

func (r *M20260930000007AddDeploymentRollbackOf) Signature() string {
	return "20260930000007_add_deployment_rollback_of"
}

func (r *M20260930000007AddDeploymentRollbackOf) Up() error {
	return facades.Schema().Sql(`ALTER TABLE deployments
		ADD COLUMN rollback_of bigint REFERENCES deployments (id) ON DELETE SET NULL`)
}

func (r *M20260930000007AddDeploymentRollbackOf) Down() error {
	return facades.Schema().DropColumns("deployments", []string{"rollback_of"})
}
