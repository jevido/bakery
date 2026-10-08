package migrations

// M20260930000091AddRunWake adds what Wakes tell a Run (agents): its Wake
// reason, how many Wakes it gathered and the comments they carried. One
// Agent has at most one queued Run per Issue (and one without an Issue),
// which is what makes a Wake join that Run instead of queueing another;
// queued twins from before are cancelled first, keeping the oldest.
type M20260930000091AddRunWake struct{}

func (r *M20260930000091AddRunWake) Signature() string {
	return "20260930000091_add_run_wake"
}

func (r *M20260930000091AddRunWake) Up() error {
	return sqls(
		`ALTER TABLE runs ADD COLUMN wake_reason text NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE runs ADD COLUMN wake_count integer NOT NULL DEFAULT 1`,
		`ALTER TABLE runs ADD COLUMN wake_context jsonb NOT NULL DEFAULT '{}'`,
		`UPDATE runs SET status = 'cancelled', finished_at = now(), updated_at = now(),
			error = 'joined an earlier queued run of the same agent and issue'
			WHERE status = 'queued' AND EXISTS (SELECT 1 FROM runs older WHERE older.status = 'queued'
				AND older.agent_id = runs.agent_id AND COALESCE(older.issue_id, 0) = COALESCE(runs.issue_id, 0)
				AND (older.created_at, older.id) < (runs.created_at, runs.id))`,
		`CREATE UNIQUE INDEX runs_agent_id_issue_id_queued_unique ON runs (agent_id, (COALESCE(issue_id, 0))) WHERE status = 'queued'`,
	)
}

func (r *M20260930000091AddRunWake) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS runs_agent_id_issue_id_queued_unique`,
		`ALTER TABLE runs DROP COLUMN IF EXISTS wake_context`,
		`ALTER TABLE runs DROP COLUMN IF EXISTS wake_count`,
		`ALTER TABLE runs DROP COLUMN IF EXISTS wake_reason`,
	)
}
