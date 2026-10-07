package migrations

// M20260930000073AddManageWorkToMemberRoles gives every Guild's Member Role
// manage_work (8192), so Members can write Goals and Issues as they could
// change everything else. Only Member Roles still holding the seeded Member
// Permissions (view_resources 2, see_secrets 4, deploy 8,
// manage_applications 16) get it: a Role a Guild has made its own keeps
// what it was given. The bits are written out, as in migration 068.
type M20260930000073AddManageWorkToMemberRoles struct{}

func (r *M20260930000073AddManageWorkToMemberRoles) Signature() string {
	return "20260930000073_add_manage_work_to_member_roles"
}

func (r *M20260930000073AddManageWorkToMemberRoles) Up() error {
	return sqls(`UPDATE roles SET permissions = permissions | 8192
		WHERE NOT base AND name = 'Member' AND permissions & 30 = 30`)
}

func (r *M20260930000073AddManageWorkToMemberRoles) Down() error {
	return sqls(`UPDATE roles SET permissions = permissions & ~8192::bigint
		WHERE NOT base AND name = 'Member' AND permissions & 30 = 30`)
}
