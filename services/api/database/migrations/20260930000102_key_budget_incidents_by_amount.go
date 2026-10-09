package migrations

// M20260930000102KeyBudgetIncidentsByAmount lets a raised Budget open a
// new incident in the same window: one not dismissed per Budget, window,
// threshold and amount, so the incident a raise resolved no longer holds
// the threshold at the new amount.
type M20260930000102KeyBudgetIncidentsByAmount struct{}

func (r *M20260930000102KeyBudgetIncidentsByAmount) Signature() string {
	return "20260930000102_key_budget_incidents_by_amount"
}

func (r *M20260930000102KeyBudgetIncidentsByAmount) Up() error {
	return sqls(
		`DROP INDEX budget_incidents_one_per_window`,
		`CREATE UNIQUE INDEX budget_incidents_one_per_window ON budget_incidents (budget_id, window_start, threshold, amount_limit)
			NULLS NOT DISTINCT WHERE status <> 'dismissed'`,
	)
}

func (r *M20260930000102KeyBudgetIncidentsByAmount) Down() error {
	return sqls(
		`DROP INDEX budget_incidents_one_per_window`,
		`DELETE FROM budget_incidents i USING budget_incidents o
			WHERE i.budget_id = o.budget_id AND i.window_start IS NOT DISTINCT FROM o.window_start AND i.threshold = o.threshold
			AND i.status <> 'dismissed' AND o.status <> 'dismissed' AND i.id < o.id`,
		`CREATE UNIQUE INDEX budget_incidents_one_per_window ON budget_incidents (budget_id, window_start, threshold)
			NULLS NOT DISTINCT WHERE status <> 'dismissed'`,
	)
}
