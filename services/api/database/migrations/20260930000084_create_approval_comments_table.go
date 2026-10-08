package migrations

// M20260930000084CreateApprovalCommentsTable stores the Approval comments
// (work): Members' messages on an Approval, never edited or deleted.
type M20260930000084CreateApprovalCommentsTable struct{}

func (r *M20260930000084CreateApprovalCommentsTable) Signature() string {
	return "20260930000084_create_approval_comments_table"
}

func (r *M20260930000084CreateApprovalCommentsTable) Up() error {
	return sqls(
		`CREATE TABLE approval_comments (
			id bigserial PRIMARY KEY,
			approval_id bigint NOT NULL REFERENCES approvals (id) ON DELETE CASCADE,
			author_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			body text NOT NULL,
			created_at timestamptz NOT NULL
		)`,
		`CREATE INDEX approval_comments_approval_id_created_at_index ON approval_comments (approval_id, created_at)`,
	)
}

func (r *M20260930000084CreateApprovalCommentsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS approval_comments`)
}
