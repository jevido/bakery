package migrations

// M20260930000100CreateBudgetsTable stores the Budgets (agents): one per
// Guild, Budget scope, Budget metric and Budget window. scope_id has no
// foreign key, as it names a Guild, an Agent or another context's
// Project; agents removes a Budget when its Agent is terminated or its
// Project deleted.
type M20260930000100CreateBudgetsTable struct{}

func (r *M20260930000100CreateBudgetsTable) Signature() string {
	return "20260930000100_create_budgets_table"
}

func (r *M20260930000100CreateBudgetsTable) Up() error {
	return sqls(
		`CREATE TABLE budgets (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			scope_type text NOT NULL CHECK (scope_type IN ('guild', 'agent', 'project')),
			scope_id bigint NOT NULL,
			metric text NOT NULL CHECK (metric IN ('tokens', 'runs', 'run_time')),
			window_kind text NOT NULL CHECK (window_kind IN ('calendar_month_utc', 'lifetime')),
			amount bigint NOT NULL CHECK (amount >= 0),
			warn_percent int NOT NULL DEFAULT 80 CHECK (warn_percent BETWEEN 1 AND 99),
			hard_stop boolean NOT NULL DEFAULT true,
			notify boolean NOT NULL DEFAULT true,
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			updated_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL,
			UNIQUE (guild_id, scope_type, scope_id, metric, window_kind)
		)`,
		`CREATE INDEX budgets_scope_type_scope_id_index ON budgets (scope_type, scope_id)`,
	)
}

func (r *M20260930000100CreateBudgetsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS budgets`)
}
