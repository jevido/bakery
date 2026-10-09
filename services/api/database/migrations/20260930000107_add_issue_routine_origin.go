package migrations

// M20260930000107AddIssueRoutineOrigin marks an Execution Issue (work)
// with the Routine and Routine run that created it, so a Routine run can
// find its Live execution Issue and follow its Issue's status.
type M20260930000107AddIssueRoutineOrigin struct{}

func (r *M20260930000107AddIssueRoutineOrigin) Signature() string {
	return "20260930000107_add_issue_routine_origin"
}

func (r *M20260930000107AddIssueRoutineOrigin) Up() error {
	return sqls(
		`ALTER TABLE issues
			ADD COLUMN origin_routine_id bigint NULL REFERENCES routines (id) ON DELETE SET NULL,
			ADD COLUMN origin_routine_run_id bigint NULL REFERENCES routine_runs (id) ON DELETE SET NULL`,
		`CREATE INDEX issues_origin_routine_id_index ON issues (origin_routine_id)`,
	)
}

func (r *M20260930000107AddIssueRoutineOrigin) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS issues_origin_routine_id_index`,
		`ALTER TABLE issues DROP COLUMN IF EXISTS origin_routine_run_id, DROP COLUMN IF EXISTS origin_routine_id`,
	)
}
