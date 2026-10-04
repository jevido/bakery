package migrations

// M20260930000046AddDescriptionToApplications lets people describe an
// Application, as Coolify's General page and API do. Existing Applications
// get an empty description.
type M20260930000046AddDescriptionToApplications struct{}

func (r *M20260930000046AddDescriptionToApplications) Signature() string {
	return "20260930000046_add_description_to_applications"
}

func (r *M20260930000046AddDescriptionToApplications) Up() error {
	return sqls(`ALTER TABLE applications ADD COLUMN description text NOT NULL DEFAULT ''`)
}

func (r *M20260930000046AddDescriptionToApplications) Down() error {
	return sqls(`ALTER TABLE applications DROP COLUMN IF EXISTS description`)
}
