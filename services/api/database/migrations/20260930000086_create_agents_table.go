package migrations

// M20260930000086CreateAgentsTable stores the Agents (agents): each
// Guild's AI workers, their Hirer, Job, Manager and Agent status.
// hire_approval_id has no foreign key, as the approvals table is work's.
type M20260930000086CreateAgentsTable struct{}

func (r *M20260930000086CreateAgentsTable) Signature() string {
	return "20260930000086_create_agents_table"
}

func (r *M20260930000086CreateAgentsTable) Up() error {
	return sqls(
		`CREATE TABLE agents (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			hirer_member_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			name text NOT NULL,
			job text NOT NULL,
			title text NOT NULL DEFAULT '',
			icon text NOT NULL DEFAULT '',
			capabilities text NOT NULL DEFAULT '',
			manager_agent_id bigint REFERENCES agents (id) ON DELETE SET NULL,
			status text NOT NULL,
			hire_approval_id bigint,
			paused_at timestamptz,
			terminated_at timestamptz,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL
		)`,
		`CREATE INDEX agents_guild_id_status_index ON agents (guild_id, status)`,
		`CREATE INDEX agents_guild_id_manager_agent_id_index ON agents (guild_id, manager_agent_id)`,
		`CREATE UNIQUE INDEX agents_guild_id_name_unique ON agents (guild_id, lower(name)) WHERE status <> 'terminated'`,
	)
}

func (r *M20260930000086CreateAgentsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS agents`)
}
