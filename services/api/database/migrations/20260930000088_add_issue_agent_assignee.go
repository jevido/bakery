package migrations

// M20260930000088AddIssueAgentAssignee lets an Issue be assigned to an
// Agent (work): assignee_agent_id beside assignee_member_id, never both.
type M20260930000088AddIssueAgentAssignee struct{}

func (r *M20260930000088AddIssueAgentAssignee) Signature() string {
	return "20260930000088_add_issue_agent_assignee"
}

func (r *M20260930000088AddIssueAgentAssignee) Up() error {
	return sqls(
		`ALTER TABLE issues ADD COLUMN assignee_agent_id bigint REFERENCES agents (id) ON DELETE SET NULL`,
		`ALTER TABLE issues ADD CONSTRAINT issues_one_assignee CHECK (assignee_member_id IS NULL OR assignee_agent_id IS NULL)`,
		`CREATE INDEX issues_guild_id_assignee_agent_id_index ON issues (guild_id, assignee_agent_id)`,
	)
}

func (r *M20260930000088AddIssueAgentAssignee) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS issues_guild_id_assignee_agent_id_index`,
		`ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_one_assignee`,
		`ALTER TABLE issues DROP COLUMN IF EXISTS assignee_agent_id`,
	)
}
