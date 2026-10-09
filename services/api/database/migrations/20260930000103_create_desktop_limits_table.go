package migrations

// M20260930000103CreateDesktopLimitsTable keeps a limited Run's reset time
// and each Desktop's Desktop limit, so a Desktop at its Subscription limit
// is refused claims until its Limit reset.
type M20260930000103CreateDesktopLimitsTable struct{}

func (r *M20260930000103CreateDesktopLimitsTable) Signature() string {
	return "20260930000103_create_desktop_limits_table"
}

func (r *M20260930000103CreateDesktopLimitsTable) Up() error {
	return sqls(
		`ALTER TABLE runs ADD COLUMN limit_resets_at timestamptz NULL`,
		`CREATE TABLE desktop_limits (
			desktop_id bigint PRIMARY KEY REFERENCES desktops (id) ON DELETE CASCADE,
			member_id bigint NOT NULL,
			resets_at timestamptz NOT NULL,
			reported_at timestamptz NOT NULL
		)`,
		`CREATE INDEX desktop_limits_member_id_index ON desktop_limits (member_id)`,
	)
}

func (r *M20260930000103CreateDesktopLimitsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS desktop_limits`, `ALTER TABLE runs DROP COLUMN IF EXISTS limit_resets_at`)
}
