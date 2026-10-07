package migrations

// M20260930000076CreateIssuesTables stores each Guild's Issues (work) and
// the counter that numbers them. project_id has no foreign key: projects is
// another context, which tells work when a Project is deleted
// (projects.OnProjectDeleted) and work clears it then. A Guild with Issues
// is not deleted (work registers guilds.OnGuildDeleting), hence RESTRICT.
type M20260930000076CreateIssuesTables struct{}

func (r *M20260930000076CreateIssuesTables) Signature() string {
	return "20260930000076_create_issues_tables"
}

func (r *M20260930000076CreateIssuesTables) Up() error {
	return sqls(
		`CREATE TABLE issues (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE RESTRICT,
			number int NOT NULL,
			title varchar(200) NOT NULL,
			description text NOT NULL DEFAULT '',
			status varchar(12) NOT NULL DEFAULT 'backlog',
			priority varchar(8) NOT NULL DEFAULT 'medium',
			assignee_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			project_id bigint NULL,
			goal_id bigint REFERENCES goals (id) ON DELETE SET NULL,
			parent_id bigint REFERENCES issues (id) ON DELETE SET NULL,
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			started_at timestamptz NULL,
			completed_at timestamptz NULL,
			cancelled_at timestamptz NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL,
			UNIQUE (guild_id, number)
		)`,
		`CREATE INDEX issues_guild_id_status_index ON issues (guild_id, status)`,
		`CREATE INDEX issues_guild_id_assignee_member_id_index ON issues (guild_id, assignee_member_id)`,
		`CREATE INDEX issues_guild_id_project_id_index ON issues (guild_id, project_id)`,
		`CREATE INDEX issues_guild_id_updated_at_index ON issues (guild_id, updated_at)`,
		`CREATE INDEX issues_goal_id_index ON issues (goal_id)`,
		`CREATE INDEX issues_parent_id_index ON issues (parent_id)`,
		`CREATE TABLE issue_counters (
			guild_id bigint PRIMARY KEY REFERENCES guilds (id) ON DELETE CASCADE,
			last_number int NOT NULL
		)`,
	)
}

func (r *M20260930000076CreateIssuesTables) Down() error {
	return sqls(`DROP TABLE IF EXISTS issue_counters`, `DROP TABLE IF EXISTS issues`)
}
