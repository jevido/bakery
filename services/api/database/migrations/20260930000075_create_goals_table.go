package migrations

// M20260930000075CreateGoalsTable stores each Guild's Goals (work), a tree
// through parent_id. A Guild with Goals is not deleted (work registers
// guilds.OnGuildDeleting), hence RESTRICT; deleting a Goal moves its
// Sub-goals up in the same transaction, so SET NULL is only a backstop.
type M20260930000075CreateGoalsTable struct{}

func (r *M20260930000075CreateGoalsTable) Signature() string {
	return "20260930000075_create_goals_table"
}

func (r *M20260930000075CreateGoalsTable) Up() error {
	return sqls(
		`CREATE TABLE goals (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE RESTRICT,
			parent_id bigint REFERENCES goals (id) ON DELETE SET NULL,
			owner_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			title varchar(200) NOT NULL,
			description text NOT NULL DEFAULT '',
			level varchar(8) NOT NULL DEFAULT 'task',
			status varchar(12) NOT NULL DEFAULT 'planned',
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
		`CREATE INDEX goals_guild_id_index ON goals (guild_id)`,
		`CREATE INDEX goals_parent_id_index ON goals (parent_id)`,
	)
}

func (r *M20260930000075CreateGoalsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS goals`)
}
