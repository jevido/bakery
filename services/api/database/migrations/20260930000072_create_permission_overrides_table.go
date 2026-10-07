package migrations

// M20260930000072CreatePermissionOverridesTable stores the Permission
// overrides of each Project (guilds), for one Role or one Member each.
// project_id has no foreign key: projects is another context's table, and
// projects tells guilds when a Project is deleted (guilds.ForgetProject).
type M20260930000072CreatePermissionOverridesTable struct{}

func (r *M20260930000072CreatePermissionOverridesTable) Signature() string {
	return "20260930000072_create_permission_overrides_table"
}

func (r *M20260930000072CreatePermissionOverridesTable) Up() error {
	return sqls(
		`CREATE TABLE permission_overrides (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			project_id bigint NOT NULL,
			role_id bigint REFERENCES roles (id) ON DELETE CASCADE,
			member_id bigint REFERENCES users (id) ON DELETE CASCADE,
			allow bigint NOT NULL DEFAULT 0,
			deny bigint NOT NULL DEFAULT 0,
			created_at timestamptz NULL,
			updated_at timestamptz NULL,
			CHECK ((role_id IS NULL) <> (member_id IS NULL))
		)`,
		`CREATE UNIQUE INDEX permission_overrides_project_role ON permission_overrides (project_id, role_id) WHERE role_id IS NOT NULL`,
		`CREATE UNIQUE INDEX permission_overrides_project_member ON permission_overrides (project_id, member_id) WHERE member_id IS NOT NULL`,
		`CREATE INDEX permission_overrides_guild_id_index ON permission_overrides (guild_id)`,
		`CREATE INDEX permission_overrides_role_id_index ON permission_overrides (role_id)`,
		`CREATE INDEX permission_overrides_member_id_index ON permission_overrides (member_id)`,
	)
}

func (r *M20260930000072CreatePermissionOverridesTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS permission_overrides`)
}
