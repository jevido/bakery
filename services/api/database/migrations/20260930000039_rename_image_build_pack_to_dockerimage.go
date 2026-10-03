package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000039RenameImageBuildPackToDockerimage gives the pulled-image
// Build pack Coolify's value, dockerimage, and its column the glossary's
// name, docker_image. Only applications stores a Build pack.
type M20260930000039RenameImageBuildPackToDockerimage struct{}

func (r *M20260930000039RenameImageBuildPackToDockerimage) Signature() string {
	return "20260930000039_rename_image_build_pack_to_dockerimage"
}

func (r *M20260930000039RenameImageBuildPackToDockerimage) Up() error {
	if err := facades.Schema().Sql(`UPDATE applications SET build_pack = 'dockerimage' WHERE build_pack = 'image'`); err != nil {
		return err
	}
	return facades.Schema().Sql(`ALTER TABLE applications RENAME COLUMN image_reference TO docker_image`)
}

func (r *M20260930000039RenameImageBuildPackToDockerimage) Down() error {
	if err := facades.Schema().Sql(`ALTER TABLE applications RENAME COLUMN docker_image TO image_reference`); err != nil {
		return err
	}
	return facades.Schema().Sql(`UPDATE applications SET build_pack = 'image' WHERE build_pack = 'dockerimage'`)
}
