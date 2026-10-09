package migrations

// M20260930000101CreateBudgetIncidentsTable gives a paused Agent its Pause
// reason (agents) and stores Budget incidents. approval_id is work's
// Approval, without a foreign key: another context's table. A lifetime
// Budget's incidents have no window bounds, so the one-per-window rule
// treats NULLs as equal.
type M20260930000101CreateBudgetIncidentsTable struct{}

func (r *M20260930000101CreateBudgetIncidentsTable) Signature() string {
	return "20260930000101_create_budget_incidents_table"
}

func (r *M20260930000101CreateBudgetIncidentsTable) Up() error {
	return sqls(
		`ALTER TABLE agents ADD COLUMN pause_reason text NULL CHECK (pause_reason IN ('manual', 'budget'))`,
		`UPDATE agents SET pause_reason = 'manual' WHERE status = 'paused'`,
		`CREATE TABLE budget_incidents (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			budget_id bigint NOT NULL REFERENCES budgets (id) ON DELETE CASCADE,
			scope_type text NOT NULL CHECK (scope_type IN ('guild', 'agent', 'project')),
			scope_id bigint NOT NULL,
			metric text NOT NULL CHECK (metric IN ('tokens', 'runs', 'run_time')),
			window_kind text NOT NULL CHECK (window_kind IN ('calendar_month_utc', 'lifetime')),
			window_start timestamptz NULL,
			window_end timestamptz NULL,
			threshold text NOT NULL CHECK (threshold IN ('soft', 'hard')),
			amount_limit bigint NOT NULL,
			amount_observed bigint NOT NULL,
			status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
			approval_id bigint NULL,
			resolved_at timestamptz NULL,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL
		)`,
		`CREATE UNIQUE INDEX budget_incidents_one_per_window ON budget_incidents (budget_id, window_start, threshold)
			NULLS NOT DISTINCT WHERE status <> 'dismissed'`,
		`CREATE INDEX budget_incidents_guild_id_status_index ON budget_incidents (guild_id, status)`,
	)
}

func (r *M20260930000101CreateBudgetIncidentsTable) Down() error {
	return sqls(
		`DROP TABLE IF EXISTS budget_incidents`,
		`ALTER TABLE agents DROP COLUMN IF EXISTS pause_reason`,
	)
}
