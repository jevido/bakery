package migrations

// M20260930000055AddInstanceAdminToUsers marks the Instance admin
// (identity); the partial index keeps it exactly one, as users_one_owner
// does for the Owner until users.role goes.
type M20260930000055AddInstanceAdminToUsers struct{}

func (r *M20260930000055AddInstanceAdminToUsers) Signature() string {
	return "20260930000055_add_instance_admin_to_users"
}

func (r *M20260930000055AddInstanceAdminToUsers) Up() error {
	return sqls(
		`ALTER TABLE users ADD COLUMN instance_admin boolean NOT NULL DEFAULT false`,
		`CREATE UNIQUE INDEX users_one_instance_admin ON users (instance_admin) WHERE instance_admin`,
	)
}

func (r *M20260930000055AddInstanceAdminToUsers) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS users_one_instance_admin`,
		`ALTER TABLE users DROP COLUMN IF EXISTS instance_admin`,
	)
}
