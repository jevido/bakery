package migrations

// M20260930000049AddDescriptionToServices lets people describe a Service,
// as Coolify's Service General page does. Existing Services get an empty
// description.
type M20260930000049AddDescriptionToServices struct{}

func (r *M20260930000049AddDescriptionToServices) Signature() string {
	return "20260930000049_add_description_to_services"
}

func (r *M20260930000049AddDescriptionToServices) Up() error {
	return sqls(`ALTER TABLE services ADD COLUMN description text NOT NULL DEFAULT ''`)
}

func (r *M20260930000049AddDescriptionToServices) Down() error {
	return sqls(`ALTER TABLE services DROP COLUMN IF EXISTS description`)
}
