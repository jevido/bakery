package migrations

// M20260930000106CreateRoutineRunsTable stores each firing of a Routine
// (work): its Routine run, what it was triggered by and the Execution
// Issue it created or was coalesced into. A Routine run goes with its
// Routine; its trigger and Issue may go before it.
type M20260930000106CreateRoutineRunsTable struct{}

func (r *M20260930000106CreateRoutineRunsTable) Signature() string {
	return "20260930000106_create_routine_runs_table"
}

func (r *M20260930000106CreateRoutineRunsTable) Up() error {
	return sqls(
		`CREATE TABLE routine_runs (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			routine_id bigint NOT NULL REFERENCES routines (id) ON DELETE CASCADE,
			trigger_id bigint NULL REFERENCES routine_triggers (id) ON DELETE SET NULL,
			source varchar(16) NOT NULL,
			status varchar(16) NOT NULL DEFAULT 'received',
			triggered_at timestamptz NOT NULL,
			linked_issue_id bigint NULL REFERENCES issues (id) ON DELETE SET NULL,
			coalesced_into_routine_run_id bigint NULL REFERENCES routine_runs (id) ON DELETE SET NULL,
			failure_reason text NOT NULL DEFAULT '',
			triggered_by_member_id bigint NULL REFERENCES users (id) ON DELETE SET NULL,
			triggered_by_agent_id bigint NULL REFERENCES agents (id),
			completed_at timestamptz NULL,
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
		`CREATE INDEX routine_runs_guild_id_routine_id_created_at_index ON routine_runs (guild_id, routine_id, created_at)`,
		`CREATE INDEX routine_runs_linked_issue_id_index ON routine_runs (linked_issue_id)`,
	)
}

func (r *M20260930000106CreateRoutineRunsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS routine_runs`)
}
