package migrations

// M20260930000078CreateIssueBlockersTable stores which Issues block which
// (work): issue_id waits on blocker_id. Both are Issues of guild_id, and
// deleting either Issue takes the row with it.
type M20260930000078CreateIssueBlockersTable struct{}

func (r *M20260930000078CreateIssueBlockersTable) Signature() string {
	return "20260930000078_create_issue_blockers_table"
}

func (r *M20260930000078CreateIssueBlockersTable) Up() error {
	return sqls(
		`CREATE TABLE issue_blockers (
			guild_id bigint NOT NULL,
			issue_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			blocker_id bigint NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (issue_id, blocker_id)
		)`,
		`CREATE INDEX issue_blockers_blocker_id_index ON issue_blockers (blocker_id)`,
	)
}

func (r *M20260930000078CreateIssueBlockersTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS issue_blockers`)
}
