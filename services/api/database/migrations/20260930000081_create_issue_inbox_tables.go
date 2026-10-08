package migrations

// M20260930000081CreateIssueInboxTables stores each Member's Inbox (work):
// their Read mark and Inbox archive per Issue, at most one of each.
type M20260930000081CreateIssueInboxTables struct{}

func (r *M20260930000081CreateIssueInboxTables) Signature() string {
	return "20260930000081_create_issue_inbox_tables"
}

func (r *M20260930000081CreateIssueInboxTables) Up() error {
	return sqls(
		`CREATE TABLE issue_read_states (
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			member_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			last_read_at timestamptz NOT NULL,
			PRIMARY KEY (issue_id, member_id)
		)`,
		`CREATE INDEX issue_read_states_member_id_index ON issue_read_states (member_id)`,
		`CREATE TABLE issue_inbox_archives (
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			member_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			archived_at timestamptz NOT NULL,
			PRIMARY KEY (issue_id, member_id)
		)`,
		`CREATE INDEX issue_inbox_archives_member_id_index ON issue_inbox_archives (member_id)`,
	)
}

func (r *M20260930000081CreateIssueInboxTables) Down() error {
	return sqls(`DROP TABLE IF EXISTS issue_inbox_archives`, `DROP TABLE IF EXISTS issue_read_states`)
}
