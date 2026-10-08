package migrations

// M20260930000089CreateRunsTables stores the Runs (agents): each execution
// of an Agent on its Hirer's Desktop, with its Run usage, and the Run
// events it reported. issue_id has no cascade, as the issues table is
// work's: a deleted Issue leaves its Runs without one.
type M20260930000089CreateRunsTables struct{}

func (r *M20260930000089CreateRunsTables) Signature() string {
	return "20260930000089_create_runs_tables"
}

func (r *M20260930000089CreateRunsTables) Up() error {
	return sqls(
		`CREATE TABLE runs (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			agent_id bigint NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
			issue_id bigint REFERENCES issues (id) ON DELETE SET NULL,
			invocation_source text NOT NULL DEFAULT 'on_demand',
			status text NOT NULL,
			requested_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			desktop_id bigint REFERENCES desktops (id) ON DELETE SET NULL,
			retry_of_run_id bigint REFERENCES runs (id) ON DELETE SET NULL,
			prompt text NOT NULL DEFAULT '',
			session_id text NOT NULL DEFAULT '',
			exit_code int,
			error text NOT NULL DEFAULT '',
			input_tokens bigint NOT NULL DEFAULT 0,
			cached_input_tokens bigint NOT NULL DEFAULT 0,
			output_tokens bigint NOT NULL DEFAULT 0,
			turns bigint NOT NULL DEFAULT 0,
			cost_equivalent_usd numeric(12,6) NOT NULL DEFAULT 0,
			duration_ms bigint NOT NULL DEFAULT 0,
			next_seq bigint NOT NULL DEFAULT 1,
			lease_expires_at timestamptz,
			created_at timestamptz NOT NULL,
			started_at timestamptz,
			finished_at timestamptz,
			updated_at timestamptz NOT NULL
		)`,
		`CREATE UNIQUE INDEX runs_agent_id_running_unique ON runs (agent_id) WHERE status = 'running'`,
		`CREATE INDEX runs_guild_id_issue_id_created_at_index ON runs (guild_id, issue_id, created_at)`,
		`CREATE INDEX runs_agent_id_status_created_at_index ON runs (agent_id, status, created_at)`,
		`CREATE TABLE run_events (
			id bigserial PRIMARY KEY,
			run_id bigint NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
			seq bigint NOT NULL,
			kind text NOT NULL,
			payload jsonb NOT NULL DEFAULT '{}',
			created_at timestamptz NOT NULL,
			UNIQUE (run_id, seq)
		)`,
	)
}

func (r *M20260930000089CreateRunsTables) Down() error {
	return sqls(`DROP TABLE IF EXISTS run_events`, `DROP TABLE IF EXISTS runs`)
}
