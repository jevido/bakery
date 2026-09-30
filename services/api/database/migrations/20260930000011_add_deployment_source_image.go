package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000011AddDeploymentSourceImage records the reference with digest
// an image Deployment pulled.
type M20260930000011AddDeploymentSourceImage struct{}

func (r *M20260930000011AddDeploymentSourceImage) Signature() string {
	return "20260930000011_add_deployment_source_image"
}

func (r *M20260930000011AddDeploymentSourceImage) Up() error {
	return facades.Schema().Sql(`ALTER TABLE deployments
		ADD COLUMN source_image varchar(600) NOT NULL DEFAULT ''`)
}

func (r *M20260930000011AddDeploymentSourceImage) Down() error {
	return facades.Schema().DropColumns("deployments", []string{"source_image"})
}
