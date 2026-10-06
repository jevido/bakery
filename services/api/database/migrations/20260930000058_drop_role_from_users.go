package migrations

// M20260930000058DropRoleFromUsers moves the last of the Roles to guilds:
// a Role is now a Membership's. Members invited or given another Role after
// the first Guild was made (identity still wrote only users.role then) get
// their Membership in the first Guild brought in line first.
type M20260930000058DropRoleFromUsers struct{}

func (r *M20260930000058DropRoleFromUsers) Signature() string {
	return "20260930000058_drop_role_from_users"
}

func (r *M20260930000058DropRoleFromUsers) Up() error {
	return sqls(
		`INSERT INTO memberships (guild_id, user_id, role, created_at, updated_at)
		SELECT (SELECT min(id) FROM guilds), u.id, CASE u.role WHEN 'owner' THEN 'admin' ELSE u.role END, now(), now()
		FROM users u
		WHERE EXISTS (SELECT 1 FROM guilds) AND NOT EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.id)`,
		`UPDATE memberships m SET role = CASE u.role WHEN 'owner' THEN 'admin' ELSE u.role END, updated_at = now()
		FROM users u
		WHERE m.user_id = u.id AND m.guild_id = (SELECT min(id) FROM guilds)
			AND m.role <> CASE u.role WHEN 'owner' THEN 'admin' ELSE u.role END`,
		`DROP INDEX IF EXISTS users_one_owner`,
		`ALTER TABLE users DROP COLUMN IF EXISTS role`,
	)
}

// Down gives users.role back from the Memberships of the first Guild: the
// Instance admin is the owner again, anyone without a Membership there a
// viewer.
func (r *M20260930000058DropRoleFromUsers) Down() error {
	return sqls(
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS role varchar(16) NOT NULL DEFAULT 'member'`,
		`UPDATE users u SET role = CASE
			WHEN u.instance_admin THEN 'owner'
			ELSE COALESCE((SELECT m.role FROM memberships m WHERE m.user_id = u.id AND m.guild_id = (SELECT min(id) FROM guilds)), 'viewer')
		END`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_one_owner ON users (role) WHERE role = 'owner'`,
	)
}
