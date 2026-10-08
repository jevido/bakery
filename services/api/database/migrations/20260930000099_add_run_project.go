package migrations

// M20260930000099AddRunProject remembers the Project a Run worked for, set
// when its Desktop claims it on an Issue in a Project, so Costs and Budgets
// count per Project even after the Issue moves. No foreign key: projects
// are another context's table.
type M20260930000099AddRunProject struct{}

func (r *M20260930000099AddRunProject) Signature() string {
	return "20260930000099_add_run_project"
}

func (r *M20260930000099AddRunProject) Up() error {
	return sqls(
		`ALTER TABLE runs ADD COLUMN project_id bigint NULL`,
		`CREATE INDEX runs_guild_id_project_id_finished_at_index ON runs (guild_id, project_id, finished_at)`,
		`CREATE INDEX runs_guild_id_finished_at_index ON runs (guild_id, finished_at)`,
	)
}

func (r *M20260930000099AddRunProject) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS runs_guild_id_finished_at_index`,
		`DROP INDEX IF EXISTS runs_guild_id_project_id_finished_at_index`,
		`ALTER TABLE runs DROP COLUMN IF EXISTS project_id`,
	)
}
