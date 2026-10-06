package migrations

// M20260930000053CreateGuildsTable holds the Guilds (guilds): Coolify's
// Teams, Paperclip's Companies.
type M20260930000053CreateGuildsTable struct{}

func (r *M20260930000053CreateGuildsTable) Signature() string {
	return "20260930000053_create_guilds_table"
}

func (r *M20260930000053CreateGuildsTable) Up() error {
	return sqls(
		`CREATE TABLE guilds (
			id bigserial PRIMARY KEY,
			name varchar(255) NOT NULL,
			description text NOT NULL DEFAULT '',
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
	)
}

func (r *M20260930000053CreateGuildsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS guilds`)
}
