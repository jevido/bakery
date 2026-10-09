package migrations

// M20260930000105CreateRoutineTriggersTable stores each Routine's Routine
// triggers (work). They go with their Routine. next_run_at is indexed for
// the scheduler, which claims the due ones by it.
type M20260930000105CreateRoutineTriggersTable struct{}

func (r *M20260930000105CreateRoutineTriggersTable) Signature() string {
	return "20260930000105_create_routine_triggers_table"
}

func (r *M20260930000105CreateRoutineTriggersTable) Up() error {
	return sqls(
		`CREATE TABLE routine_triggers (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			routine_id bigint NOT NULL REFERENCES routines (id) ON DELETE CASCADE,
			kind varchar(16) NOT NULL,
			label varchar(100) NOT NULL DEFAULT '',
			enabled boolean NOT NULL DEFAULT true,
			cron_expression varchar(255) NOT NULL DEFAULT '',
			timezone varchar(64) NOT NULL DEFAULT '',
			next_run_at timestamptz NULL,
			last_fired_at timestamptz NULL,
			last_result text NOT NULL DEFAULT '',
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_by_agent_id bigint REFERENCES agents (id),
			created_at timestamptz NULL,
			updated_at timestamptz NULL
		)`,
		`CREATE INDEX routine_triggers_next_run_at_index ON routine_triggers (next_run_at)`,
		`CREATE INDEX routine_triggers_guild_id_routine_id_index ON routine_triggers (guild_id, routine_id)`,
	)
}

func (r *M20260930000105CreateRoutineTriggersTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS routine_triggers`)
}
