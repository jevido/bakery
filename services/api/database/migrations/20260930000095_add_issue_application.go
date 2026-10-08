package migrations

// M20260930000095AddIssueApplication gives an Issue its Application: one
// Application of its Project, which a Run on the Issue works in. Like
// project_id it has no foreign key: applications is another context, which
// tells work when an Application is deleted (projects.OnApplicationDeleted)
// and work clears it then.
type M20260930000095AddIssueApplication struct{}

func (r *M20260930000095AddIssueApplication) Signature() string {
	return "20260930000095_add_issue_application"
}

func (r *M20260930000095AddIssueApplication) Up() error {
	return sqls(
		`ALTER TABLE issues ADD COLUMN application_id bigint NULL`,
		`CREATE INDEX issues_application_id_index ON issues (application_id) WHERE application_id IS NOT NULL`,
	)
}

func (r *M20260930000095AddIssueApplication) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS issues_application_id_index`,
		`ALTER TABLE issues DROP COLUMN IF EXISTS application_id`,
	)
}
