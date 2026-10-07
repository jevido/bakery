package migrations

// M20260930000077CreateIssueCommentsTable stores the Comments on Issues
// (work). A Comment goes with its Issue; one whose author's account is
// deleted stays, without an author.
type M20260930000077CreateIssueCommentsTable struct{}

func (r *M20260930000077CreateIssueCommentsTable) Signature() string {
	return "20260930000077_create_issue_comments_table"
}

func (r *M20260930000077CreateIssueCommentsTable) Up() error {
	return sqls(
		`CREATE TABLE issue_comments (
			id bigserial PRIMARY KEY,
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			author_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			body text NOT NULL,
			deleted_at timestamptz NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
		`CREATE INDEX issue_comments_issue_id_created_at_index ON issue_comments (issue_id, created_at)`,
	)
}

func (r *M20260930000077CreateIssueCommentsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS issue_comments`)
}
