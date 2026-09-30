package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000010AddApplicationRegistryCredentials stores the Registry
// credentials of image Applications, the password encrypted with APP_KEY.
type M20260930000010AddApplicationRegistryCredentials struct{}

func (r *M20260930000010AddApplicationRegistryCredentials) Signature() string {
	return "20260930000010_add_application_registry_credentials"
}

func (r *M20260930000010AddApplicationRegistryCredentials) Up() error {
	return facades.Schema().Sql(`ALTER TABLE applications
		ADD COLUMN registry_username varchar(255) NOT NULL DEFAULT '',
		ADD COLUMN registry_password_encrypted text NOT NULL DEFAULT ''`)
}

func (r *M20260930000010AddApplicationRegistryCredentials) Down() error {
	return facades.Schema().DropColumns("applications", []string{"registry_username", "registry_password_encrypted"})
}
