package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000003AddApplicationDeployKeys holds an Application's Deploy key:
// the public half as is, the private half encrypted with APP_KEY.
type M20260930000003AddApplicationDeployKeys struct{}

func (r *M20260930000003AddApplicationDeployKeys) Signature() string {
	return "20260930000003_add_application_deploy_keys"
}

func (r *M20260930000003AddApplicationDeployKeys) Up() error {
	return facades.Schema().Table("applications", func(t schema.Blueprint) {
		t.Text("deploy_key_public").Default("")
		t.Text("deploy_key_private_encrypted").Default("")
	})
}

func (r *M20260930000003AddApplicationDeployKeys) Down() error {
	return facades.Schema().DropColumns("applications", []string{"deploy_key_public", "deploy_key_private_encrypted"})
}
