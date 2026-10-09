package migrations

// M20260930000104CreateRoutinesTable stores each Guild's Routines (work).
// project_id has no foreign key, as issues.project_id: projects tells work
// when a Project is deleted (projects.OnProjectDeleted) and work deletes
// its Routines then. assignee_agent_id has no ON DELETE: Agents are
// terminated, never deleted, and terminating one clears it here.
type M20260930000104CreateRoutinesTable struct{}

func (r *M20260930000104CreateRoutinesTable) Signature() string {
	return "20260930000104_create_routines_table"
}

func (r *M20260930000104CreateRoutinesTable) Up() error {
	return sqls(
		`CREATE TABLE routines (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			project_id bigint NULL,
			goal_id bigint REFERENCES goals (id) ON DELETE SET NULL,
			parent_issue_id bigint REFERENCES issues (id) ON DELETE SET NULL,
			title varchar(200) NOT NULL,
			description text NOT NULL DEFAULT '',
			assignee_agent_id bigint REFERENCES agents (id),
			priority varchar(8) NOT NULL DEFAULT 'medium',
			status varchar(8) NOT NULL DEFAULT 'active',
			concurrency_policy varchar(24) NOT NULL DEFAULT 'coalesce_if_active',
			catch_up_policy varchar(32) NOT NULL DEFAULT 'skip_missed',
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_by_agent_id bigint REFERENCES agents (id),
			last_triggered_at timestamptz NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
		`CREATE INDEX routines_guild_id_status_index ON routines (guild_id, status)`,
		`CREATE INDEX routines_guild_id_assignee_agent_id_index ON routines (guild_id, assignee_agent_id)`,
		`CREATE INDEX routines_project_id_index ON routines (project_id)`,
	)
}

func (r *M20260930000104CreateRoutinesTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS routines`)
}
