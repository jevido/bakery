package migrations

// M20260930000097CreateIssueWorkProductsTable stores the Work products of
// Issues (work): a Pull request or a Preview's link, one row per Issue,
// type and external id. They go with their Issue. application_id and the
// Run and Agent that made one have no foreign key: those are other
// contexts' tables.
type M20260930000097CreateIssueWorkProductsTable struct{}

func (r *M20260930000097CreateIssueWorkProductsTable) Signature() string {
	return "20260930000097_create_issue_work_products_table"
}

func (r *M20260930000097CreateIssueWorkProductsTable) Up() error {
	return sqls(
		`CREATE TABLE issue_work_products (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL,
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			application_id bigint NOT NULL,
			type varchar(16) NOT NULL,
			provider varchar(16) NOT NULL DEFAULT '',
			external_id varchar(64) NOT NULL,
			title varchar(255) NOT NULL DEFAULT '',
			url text NOT NULL DEFAULT '',
			status varchar(16) NOT NULL,
			created_by_run_id bigint NULL,
			created_by_agent_id bigint NULL,
			created_by_member_id bigint NULL REFERENCES users (id) ON DELETE SET NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL,
			UNIQUE (issue_id, type, external_id)
		)`,
		`CREATE INDEX issue_work_products_application_index ON issue_work_products (application_id, type, external_id)`,
	)
}

func (r *M20260930000097CreateIssueWorkProductsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS issue_work_products`)
}
