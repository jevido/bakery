package migrations

// M20260930000069AddGuildMasterToGuilds gives every Guild its Guild Master
// (guilds): the Instance admin in the first Guild ("Default"), and in every
// other the Member of its earliest Membership holding the seeded Admin Role
// (its creator), else of its earliest Membership. The Instance admin gets a
// Membership holding Admin where they are made Guild Master without one, so
// the Guild Master always holds a Membership. No cascade: a Member who is a
// Guild Master cannot be deleted.
type M20260930000069AddGuildMasterToGuilds struct{}

func (r *M20260930000069AddGuildMasterToGuilds) Signature() string {
	return "20260930000069_add_guild_master_to_guilds"
}

func (r *M20260930000069AddGuildMasterToGuilds) Up() error {
	return sqls(
		`WITH added AS (
			INSERT INTO memberships (guild_id, user_id, created_at, updated_at)
			SELECT g.id, u.id, now(), now() FROM guilds g CROSS JOIN users u
			WHERE u.instance_admin
				AND (g.id = (SELECT min(id) FROM guilds)
					OR NOT EXISTS (SELECT 1 FROM memberships m WHERE m.guild_id = g.id))
				AND NOT EXISTS (SELECT 1 FROM memberships m WHERE m.guild_id = g.id AND m.user_id = u.id)
			RETURNING id, guild_id
		)
		INSERT INTO membership_roles (membership_id, role_id)
		SELECT a.id, r.id FROM added a JOIN roles r ON r.guild_id = a.guild_id AND NOT r.base AND r.name = 'Admin'`,
		`ALTER TABLE guilds ADD COLUMN master_id bigint REFERENCES users (id)`,
		`UPDATE guilds g SET master_id = u.id FROM users u
		WHERE u.instance_admin AND g.id = (SELECT min(id) FROM guilds)`,
		`UPDATE guilds g SET master_id = COALESCE(
			(SELECT m.user_id FROM memberships m
				JOIN membership_roles mr ON mr.membership_id = m.id
				JOIN roles r ON r.id = mr.role_id AND NOT r.base AND r.name = 'Admin'
				WHERE m.guild_id = g.id ORDER BY m.id LIMIT 1),
			(SELECT m.user_id FROM memberships m WHERE m.guild_id = g.id ORDER BY m.id LIMIT 1))
		WHERE master_id IS NULL`,
		`ALTER TABLE guilds ALTER COLUMN master_id SET NOT NULL`,
		`CREATE INDEX guilds_master_id_index ON guilds (master_id)`,
	)
}

func (r *M20260930000069AddGuildMasterToGuilds) Down() error {
	return sqls(`ALTER TABLE guilds DROP COLUMN IF EXISTS master_id`)
}
