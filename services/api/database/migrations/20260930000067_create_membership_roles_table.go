package migrations

// M20260930000067CreateMembershipRolesTable holds the Roles each
// Membership holds (guilds); the Base role is held by everyone and never
// stored.
type M20260930000067CreateMembershipRolesTable struct{}

func (r *M20260930000067CreateMembershipRolesTable) Signature() string {
	return "20260930000067_create_membership_roles_table"
}

func (r *M20260930000067CreateMembershipRolesTable) Up() error {
	return sqls(
		`CREATE TABLE membership_roles (
			membership_id bigint NOT NULL REFERENCES memberships (id) ON DELETE CASCADE,
			role_id bigint NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
			PRIMARY KEY (membership_id, role_id)
		)`,
		`CREATE INDEX membership_roles_role_id_index ON membership_roles (role_id)`,
	)
}

func (r *M20260930000067CreateMembershipRolesTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS membership_roles`)
}
