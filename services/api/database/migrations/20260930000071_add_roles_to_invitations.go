package migrations

// M20260930000071AddRolesToInvitations gives each Invitation the Roles it
// carries (guilds) in place of its former role: an Invitation that named
// "viewer", "member" or "admin" now carries the seeded Role of that name in
// its Guild, then the former role is dropped. A Role deleted later simply
// drops out of the Invitations that carried it.
type M20260930000071AddRolesToInvitations struct{}

func (r *M20260930000071AddRolesToInvitations) Signature() string {
	return "20260930000071_add_roles_to_invitations"
}

func (r *M20260930000071AddRolesToInvitations) Up() error {
	return sqls(
		`CREATE TABLE invitation_roles (
			invitation_id bigint NOT NULL REFERENCES invitations (id) ON DELETE CASCADE,
			role_id bigint NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
			PRIMARY KEY (invitation_id, role_id)
		)`,
		`CREATE INDEX invitation_roles_role_id_index ON invitation_roles (role_id)`,
		`INSERT INTO invitation_roles (invitation_id, role_id)
		SELECT i.id, r.id FROM invitations i
		JOIN roles r ON r.guild_id = i.guild_id AND NOT r.base AND lower(r.name) = i.role`,
		`ALTER TABLE invitations DROP COLUMN role`,
	)
}

// Down gives every Invitation back the former role of the highest seeded
// Role it carries, viewer when it carries none.
func (r *M20260930000071AddRolesToInvitations) Down() error {
	return sqls(
		`ALTER TABLE invitations ADD COLUMN role varchar(16) NOT NULL DEFAULT 'viewer'`,
		`UPDATE invitations i SET role = COALESCE((
			SELECT lower(r.name) FROM invitation_roles ir JOIN roles r ON r.id = ir.role_id
			WHERE ir.invitation_id = i.id AND NOT r.base AND r.name IN ('Viewer', 'Member', 'Admin')
			ORDER BY r.position DESC LIMIT 1), 'viewer')`,
		`DROP TABLE IF EXISTS invitation_roles`,
	)
}
