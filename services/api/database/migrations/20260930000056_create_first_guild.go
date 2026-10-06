package migrations

// M20260930000056CreateFirstGuild turns an installation from before Guilds
// into its first Guild, "Default": every Member gets a Membership with the
// Role they have (the Owner's becomes admin), and the Owner becomes the
// Instance admin. One statement, so it all happens or nothing does; it does
// nothing on a fresh database (Setup makes the first Guild there) or when a
// Guild exists already.
type M20260930000056CreateFirstGuild struct{}

func (r *M20260930000056CreateFirstGuild) Signature() string {
	return "20260930000056_create_first_guild"
}

func (r *M20260930000056CreateFirstGuild) Up() error {
	return sqls(
		`WITH g AS (
			INSERT INTO guilds (name, description, created_at, updated_at)
			SELECT 'Default', '', now(), now()
			WHERE EXISTS (SELECT 1 FROM users) AND NOT EXISTS (SELECT 1 FROM guilds)
			RETURNING id
		), m AS (
			INSERT INTO memberships (guild_id, user_id, role, created_at, updated_at)
			SELECT g.id, u.id, CASE u.role WHEN 'owner' THEN 'admin' ELSE u.role END, now(), now()
			FROM g, users u
		)
		UPDATE users SET instance_admin = true
		WHERE role = 'owner' AND EXISTS (SELECT 1 FROM g)`,
	)
}

func (r *M20260930000056CreateFirstGuild) Down() error {
	return sqls(
		`DELETE FROM memberships WHERE guild_id IN (SELECT id FROM guilds WHERE name = 'Default')`,
		`DELETE FROM guilds WHERE name = 'Default'`,
		`UPDATE users SET instance_admin = false WHERE instance_admin`,
	)
}
