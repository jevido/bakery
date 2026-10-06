package migrations

// M20260930000061AddGuildToServers makes every Remote server belong to one
// Guild; those from before Guilds belong to the first one. The Local server
// is the installation's and belongs to none, which the CHECK holds to. Names
// and addresses are unique within a Guild; "localhost" stays the Local
// server's in every Guild because servers' domain refuses it for a Remote one.
type M20260930000061AddGuildToServers struct{}

func (r *M20260930000061AddGuildToServers) Signature() string {
	return "20260930000061_add_guild_to_servers"
}

func (r *M20260930000061AddGuildToServers) Up() error {
	return sqls(
		`ALTER TABLE servers ADD COLUMN guild_id bigint REFERENCES guilds (id)`,
		`INSERT INTO guilds (name, description, created_at, updated_at)
		SELECT 'Default', '', now(), now()
		WHERE EXISTS (SELECT 1 FROM servers WHERE kind <> 'local') AND NOT EXISTS (SELECT 1 FROM guilds)`,
		`UPDATE servers SET guild_id = (SELECT min(id) FROM guilds) WHERE kind <> 'local'`,
		`ALTER TABLE servers ADD CONSTRAINT servers_guild_id_local_check CHECK ((guild_id IS NULL) = (kind = 'local'))`,
		`ALTER TABLE servers DROP CONSTRAINT servers_name_unique`,
		`ALTER TABLE servers DROP CONSTRAINT servers_host_port_user_name_unique`,
		`CREATE UNIQUE INDEX servers_guild_id_name_unique ON servers (guild_id, lower(name))`,
		`CREATE UNIQUE INDEX servers_guild_id_host_port_user_name_unique ON servers (guild_id, lower(host), port, user_name)`,
	)
}

func (r *M20260930000061AddGuildToServers) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS servers_guild_id_name_unique`,
		`DROP INDEX IF EXISTS servers_guild_id_host_port_user_name_unique`,
		`ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_guild_id_local_check`,
		`ALTER TABLE servers DROP COLUMN IF EXISTS guild_id`,
		`ALTER TABLE servers ADD CONSTRAINT servers_name_unique UNIQUE (name)`,
		`ALTER TABLE servers ADD CONSTRAINT servers_host_port_user_name_unique UNIQUE (host, port, user_name)`,
	)
}
