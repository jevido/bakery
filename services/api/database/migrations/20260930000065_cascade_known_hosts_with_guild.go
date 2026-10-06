package migrations

// M20260930000065CascadeKnownHostsWithGuild deletes a Guild's Known hosts
// with the Guild. They are what its git sources were trusted with and mean
// nothing outside it, so they never block deleting it.
type M20260930000065CascadeKnownHostsWithGuild struct{}

func (r *M20260930000065CascadeKnownHostsWithGuild) Signature() string {
	return "20260930000065_cascade_known_hosts_with_guild"
}

func (r *M20260930000065CascadeKnownHostsWithGuild) Up() error {
	return sqls(
		`ALTER TABLE known_hosts DROP CONSTRAINT known_hosts_guild_id_fkey`,
		`ALTER TABLE known_hosts ADD CONSTRAINT known_hosts_guild_id_fkey
		FOREIGN KEY (guild_id) REFERENCES guilds (id) ON DELETE CASCADE`,
	)
}

func (r *M20260930000065CascadeKnownHostsWithGuild) Down() error {
	return sqls(
		`ALTER TABLE known_hosts DROP CONSTRAINT known_hosts_guild_id_fkey`,
		`ALTER TABLE known_hosts ADD CONSTRAINT known_hosts_guild_id_fkey
		FOREIGN KEY (guild_id) REFERENCES guilds (id)`,
	)
}
