package migrations

// M20260930000064AddGuildToKnownHosts makes every Known host belong to one Guild;
// those from before Guilds belong to the first one. The host is unique
// within a Guild.
type M20260930000064AddGuildToKnownHosts struct{}

func (r *M20260930000064AddGuildToKnownHosts) Signature() string {
	return "20260930000064_add_guild_to_known_hosts"
}

func (r *M20260930000064AddGuildToKnownHosts) Up() error {
	return sqls(
		`ALTER TABLE known_hosts ADD COLUMN guild_id bigint REFERENCES guilds (id)`,
		`INSERT INTO guilds (name, description, created_at, updated_at)
		SELECT 'Default', '', now(), now()
		WHERE EXISTS (SELECT 1 FROM known_hosts) AND NOT EXISTS (SELECT 1 FROM guilds)`,
		`UPDATE known_hosts SET guild_id = (SELECT min(id) FROM guilds)`,
		`ALTER TABLE known_hosts ALTER COLUMN guild_id SET NOT NULL`,
		`ALTER TABLE known_hosts DROP CONSTRAINT known_hosts_host_unique`,
		`ALTER TABLE known_hosts ADD CONSTRAINT known_hosts_guild_id_host_unique UNIQUE (guild_id, host)`,
	)
}

func (r *M20260930000064AddGuildToKnownHosts) Down() error {
	return sqls(
		`ALTER TABLE known_hosts DROP COLUMN IF EXISTS guild_id`,
		`ALTER TABLE known_hosts ADD CONSTRAINT known_hosts_host_unique UNIQUE (host)`,
	)
}
