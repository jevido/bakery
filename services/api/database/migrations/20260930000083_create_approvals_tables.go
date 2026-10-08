package migrations

// M20260930000083CreateApprovalsTables stores the Approvals (work): each
// Guild's requests for a Board Decision and the Issues each is about. The
// payload keeps the Approval type's own fields, in snake_case.
type M20260930000083CreateApprovalsTables struct{}

func (r *M20260930000083CreateApprovalsTables) Signature() string {
	return "20260930000083_create_approvals_tables"
}

func (r *M20260930000083CreateApprovalsTables) Up() error {
	return sqls(
		`CREATE TABLE approvals (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			type text NOT NULL,
			status text NOT NULL,
			payload jsonb NOT NULL,
			requested_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			decided_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			decision_note text,
			decided_at timestamptz,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL
		)`,
		`CREATE INDEX approvals_guild_id_status_index ON approvals (guild_id, status)`,
		`CREATE TABLE approval_issues (
			approval_id bigint NOT NULL REFERENCES approvals (id) ON DELETE CASCADE,
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			created_at timestamptz NOT NULL,
			PRIMARY KEY (approval_id, issue_id)
		)`,
		`CREATE INDEX approval_issues_issue_id_index ON approval_issues (issue_id)`,
	)
}

func (r *M20260930000083CreateApprovalsTables) Down() error {
	return sqls(`DROP TABLE IF EXISTS approval_issues`, `DROP TABLE IF EXISTS approvals`)
}
