package migrations

// M20260930000047AddDescriptionToDatabases lets people describe a Database,
// as Coolify's Database General page does. Existing Databases get an empty
// description.
type M20260930000047AddDescriptionToDatabases struct{}

func (r *M20260930000047AddDescriptionToDatabases) Signature() string {
	return "20260930000047_add_description_to_databases"
}

func (r *M20260930000047AddDescriptionToDatabases) Up() error {
	return sqls(`ALTER TABLE databases ADD COLUMN description text NOT NULL DEFAULT ''`)
}

func (r *M20260930000047AddDescriptionToDatabases) Down() error {
	return sqls(`ALTER TABLE databases DROP COLUMN IF EXISTS description`)
}
