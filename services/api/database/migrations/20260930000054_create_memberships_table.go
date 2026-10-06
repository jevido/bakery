package migrations

// M20260930000054CreateMembershipsTable holds each Member's Role in a Guild
// (guilds), at most one per Member and Guild.
type M20260930000054CreateMembershipsTable struct{}

func (r *M20260930000054CreateMembershipsTable) Signature() string {
	return "20260930000054_create_memberships_table"
}

func (r *M20260930000054CreateMembershipsTable) Up() error {
	return sqls(
		`CREATE TABLE memberships (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			role varchar(16) NOT NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL,
			UNIQUE (guild_id, user_id)
		)`,
		`CREATE INDEX memberships_user_id_index ON memberships (user_id)`,
	)
}

func (r *M20260930000054CreateMembershipsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS memberships`)
}
