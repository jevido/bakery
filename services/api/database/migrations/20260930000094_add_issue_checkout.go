package migrations

// M20260930000094AddIssueCheckout gives an Issue its Checkout: the Run
// holding it and since when. A deleted Run holds nothing.
type M20260930000094AddIssueCheckout struct{}

func (r *M20260930000094AddIssueCheckout) Signature() string {
	return "20260930000094_add_issue_checkout"
}

func (r *M20260930000094AddIssueCheckout) Up() error {
	return sqls(
		`ALTER TABLE issues ADD COLUMN checkout_run_id bigint REFERENCES runs (id) ON DELETE SET NULL`,
		`ALTER TABLE issues ADD COLUMN checked_out_at timestamptz`,
	)
}

func (r *M20260930000094AddIssueCheckout) Down() error {
	return sqls(
		`ALTER TABLE issues DROP COLUMN IF EXISTS checked_out_at`,
		`ALTER TABLE issues DROP COLUMN IF EXISTS checkout_run_id`,
	)
}
