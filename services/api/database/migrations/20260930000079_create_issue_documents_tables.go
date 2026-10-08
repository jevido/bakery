package migrations

// M20260930000079CreateIssueDocumentsTables stores Issue documents and
// their Revisions (work). A document goes with its Issue and its Revisions
// go with it; one whose author's account is deleted stays, without an
// author. revision_number is the newest Revision's number, so a save that
// started from an older one is refused by a conditional UPDATE.
type M20260930000079CreateIssueDocumentsTables struct{}

func (r *M20260930000079CreateIssueDocumentsTables) Signature() string {
	return "20260930000079_create_issue_documents_tables"
}

func (r *M20260930000079CreateIssueDocumentsTables) Up() error {
	return sqls(
		`CREATE TABLE issue_documents (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL,
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			key varchar(64) NOT NULL,
			title varchar(200) NOT NULL DEFAULT '',
			body text NOT NULL,
			revision_number int NOT NULL,
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			updated_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL,
			UNIQUE (issue_id, key)
		)`,
		`CREATE TABLE issue_document_revisions (
			id bigserial PRIMARY KEY,
			document_id bigint NOT NULL REFERENCES issue_documents (id) ON DELETE CASCADE,
			number int NOT NULL,
			title varchar(200) NOT NULL DEFAULT '',
			body text NOT NULL,
			change_summary varchar(500) NULL,
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (document_id, number)
		)`,
	)
}

func (r *M20260930000079CreateIssueDocumentsTables) Down() error {
	return sqls(`DROP TABLE IF EXISTS issue_document_revisions`, `DROP TABLE IF EXISTS issue_documents`)
}
