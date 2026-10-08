package migrations

// M20260930000087CreateDesktopsTables stores the Desktops (identity): each
// signed-in copy of the Desktop app, its key only as a hash, and the
// Desktop sign-ins that make them, their secret only as a hash too.
type M20260930000087CreateDesktopsTables struct{}

func (r *M20260930000087CreateDesktopsTables) Signature() string {
	return "20260930000087_create_desktops_tables"
}

func (r *M20260930000087CreateDesktopsTables) Up() error {
	return sqls(
		`CREATE TABLE desktops (
			id bigserial PRIMARY KEY,
			member_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			name varchar(100) NOT NULL,
			key_hash text NOT NULL UNIQUE,
			last_seen_at timestamptz,
			revoked_at timestamptz,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL
		)`,
		`CREATE INDEX desktops_member_id_index ON desktops (member_id)`,
		`CREATE TABLE desktop_sign_ins (
			id bigserial PRIMARY KEY,
			secret_hash text NOT NULL,
			client_name varchar(100) NOT NULL,
			pending_key_hash text NOT NULL,
			approved_by_member_id bigint REFERENCES users (id) ON DELETE CASCADE,
			desktop_id bigint REFERENCES desktops (id) ON DELETE SET NULL,
			approved_at timestamptz,
			cancelled_at timestamptz,
			expires_at timestamptz NOT NULL,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL
		)`,
	)
}

func (r *M20260930000087CreateDesktopsTables) Down() error {
	return sqls(`DROP TABLE IF EXISTS desktop_sign_ins`, `DROP TABLE IF EXISTS desktops`)
}
