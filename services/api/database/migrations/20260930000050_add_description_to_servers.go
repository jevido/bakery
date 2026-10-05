package migrations

// M20260930000050AddDescriptionToServers lets people describe a Server, as
// Coolify's Server pages do. Existing Servers get an empty description.
type M20260930000050AddDescriptionToServers struct{}

func (r *M20260930000050AddDescriptionToServers) Signature() string {
	return "20260930000050_add_description_to_servers"
}

func (r *M20260930000050AddDescriptionToServers) Up() error {
	return sqls(`ALTER TABLE servers ADD COLUMN description text NOT NULL DEFAULT ''`)
}

func (r *M20260930000050AddDescriptionToServers) Down() error {
	return sqls(`ALTER TABLE servers DROP COLUMN IF EXISTS description`)
}
