package migrations

// M20260930000066CreateRolesTable holds each Guild's Roles (guilds): a
// name, a color, a Position and the Permissions as bits. Positions are
// unique per Guild but only checked at commit, so a reorder can swap them
// in one transaction; at most one Role per Guild is its Base role.
type M20260930000066CreateRolesTable struct{}

func (r *M20260930000066CreateRolesTable) Signature() string {
	return "20260930000066_create_roles_table"
}

func (r *M20260930000066CreateRolesTable) Up() error {
	return sqls(
		`CREATE TABLE roles (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			name varchar(100) NOT NULL,
			color varchar(7) NOT NULL,
			position integer NOT NULL,
			permissions bigint NOT NULL DEFAULT 0,
			base boolean NOT NULL DEFAULT false,
			created_at timestamptz NULL,
			updated_at timestamptz NULL,
			CONSTRAINT roles_guild_id_position_unique UNIQUE (guild_id, position) DEFERRABLE INITIALLY DEFERRED
		)`,
		`CREATE UNIQUE INDEX roles_one_base_per_guild ON roles (guild_id) WHERE base`,
	)
}

func (r *M20260930000066CreateRolesTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS roles`)
}
