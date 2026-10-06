package migrations

// M20260930000060AddGuildToProjects makes every Project belong to one Guild;
// the Projects from before Guilds belong to the first one. A Guild with
// Projects cannot be deleted (no cascade): it must be emptied first.
type M20260930000060AddGuildToProjects struct{}

func (r *M20260930000060AddGuildToProjects) Signature() string {
	return "20260930000060_add_guild_to_projects"
}

func (r *M20260930000060AddGuildToProjects) Up() error {
	return sqls(
		`ALTER TABLE projects ADD COLUMN guild_id bigint REFERENCES guilds (id)`,
		// Projects without any Guild means without Members: they still get
		// one, "Default", which Setup then leaves alone.
		`INSERT INTO guilds (name, description, created_at, updated_at)
		SELECT 'Default', '', now(), now()
		WHERE EXISTS (SELECT 1 FROM projects) AND NOT EXISTS (SELECT 1 FROM guilds)`,
		`UPDATE projects SET guild_id = (SELECT min(id) FROM guilds)`,
		`ALTER TABLE projects ALTER COLUMN guild_id SET NOT NULL`,
		`CREATE INDEX projects_guild_id_index ON projects (guild_id)`,
	)
}

func (r *M20260930000060AddGuildToProjects) Down() error {
	return sqls(`ALTER TABLE projects DROP COLUMN IF EXISTS guild_id`)
}
