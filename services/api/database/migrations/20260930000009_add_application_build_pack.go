package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000009AddApplicationBuildPack gives every Application a Build
// pack (existing ones build their Dockerfile), an Image reference for the
// image pack and a Publish directory for the static pack.
type M20260930000009AddApplicationBuildPack struct{}

func (r *M20260930000009AddApplicationBuildPack) Signature() string {
	return "20260930000009_add_application_build_pack"
}

func (r *M20260930000009AddApplicationBuildPack) Up() error {
	return facades.Schema().Sql(`ALTER TABLE applications
		ADD COLUMN build_pack varchar(20) NOT NULL DEFAULT 'dockerfile',
		ADD COLUMN image_reference varchar(500) NOT NULL DEFAULT '',
		ADD COLUMN publish_directory varchar(255) NOT NULL DEFAULT '.'`)
}

func (r *M20260930000009AddApplicationBuildPack) Down() error {
	return facades.Schema().DropColumns("applications", []string{"build_pack", "image_reference", "publish_directory"})
}
