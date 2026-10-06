package migrations

// M20260930000068SeedRolesFromMemberships gives every Guild the seeded
// Roles, @everyone, Viewer, Member and Admin, and every Membership the Role
// its former role named, so nobody's access changes; then drops the former
// role. The Permission bits are written out here rather than taken from the
// guilds domain, so this migration never changes when the domain does:
// administrator 1, view_resources 2, see_secrets 4, deploy 8,
// manage_applications 16.
type M20260930000068SeedRolesFromMemberships struct{}

func (r *M20260930000068SeedRolesFromMemberships) Signature() string {
	return "20260930000068_seed_roles_from_memberships"
}

func (r *M20260930000068SeedRolesFromMemberships) Up() error {
	return sqls(
		`INSERT INTO roles (guild_id, name, color, position, permissions, base, created_at, updated_at)
		SELECT g.id, s.name, s.color, s.position, s.permissions, s.base, now(), now()
		FROM guilds g CROSS JOIN (VALUES
			('@everyone', '#99aab5', 0, 0::bigint, true),
			('Viewer', '#95a5a6', 1, 2::bigint, false),
			('Member', '#3498db', 2, 30::bigint, false),
			('Admin', '#e74c3c', 3, 1::bigint, false)
		) AS s (name, color, position, permissions, base)`,
		`INSERT INTO membership_roles (membership_id, role_id)
		SELECT m.id, r.id FROM memberships m
		JOIN roles r ON r.guild_id = m.guild_id AND NOT r.base AND lower(r.name) = m.role`,
		`ALTER TABLE memberships DROP COLUMN role`,
	)
}

// Down gives every Membership back the former role of the highest seeded
// Role it holds, viewer when it holds none.
func (r *M20260930000068SeedRolesFromMemberships) Down() error {
	return sqls(
		`ALTER TABLE memberships ADD COLUMN role varchar(16) NOT NULL DEFAULT 'viewer'`,
		`UPDATE memberships m SET role = COALESCE((
			SELECT lower(r.name) FROM membership_roles mr JOIN roles r ON r.id = mr.role_id
			WHERE mr.membership_id = m.id AND NOT r.base AND r.name IN ('Viewer', 'Member', 'Admin')
			ORDER BY r.position DESC LIMIT 1
		), 'viewer')`,
		`ALTER TABLE memberships ALTER COLUMN role DROP DEFAULT`,
		`DELETE FROM membership_roles`,
		`DELETE FROM roles`,
	)
}
