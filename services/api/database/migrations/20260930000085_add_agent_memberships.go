package migrations

// M20260930000085AddAgentMemberships lets a Membership belong to an Agent
// (guilds): an Agent membership has an agent_id and its Hirer instead of a
// user_id. agent_id has no foreign key, as the agents table is the agents
// context's.
type M20260930000085AddAgentMemberships struct{}

func (r *M20260930000085AddAgentMemberships) Signature() string {
	return "20260930000085_add_agent_memberships"
}

func (r *M20260930000085AddAgentMemberships) Up() error {
	return sqls(
		`ALTER TABLE memberships ALTER COLUMN user_id DROP NOT NULL`,
		`ALTER TABLE memberships ADD COLUMN agent_id bigint`,
		`ALTER TABLE memberships ADD COLUMN hirer_member_id bigint REFERENCES users (id) ON DELETE CASCADE`,
		`ALTER TABLE memberships ADD CONSTRAINT memberships_member_or_agent CHECK (
			(user_id IS NOT NULL AND agent_id IS NULL AND hirer_member_id IS NULL)
			OR (user_id IS NULL AND agent_id IS NOT NULL AND hirer_member_id IS NOT NULL))`,
		`CREATE UNIQUE INDEX memberships_guild_id_agent_id_unique ON memberships (guild_id, agent_id) WHERE agent_id IS NOT NULL`,
	)
}

func (r *M20260930000085AddAgentMemberships) Down() error {
	return sqls(
		`DELETE FROM memberships WHERE agent_id IS NOT NULL`,
		`DROP INDEX IF EXISTS memberships_guild_id_agent_id_unique`,
		`ALTER TABLE memberships DROP CONSTRAINT IF EXISTS memberships_member_or_agent`,
		`ALTER TABLE memberships DROP COLUMN IF EXISTS hirer_member_id`,
		`ALTER TABLE memberships DROP COLUMN IF EXISTS agent_id`,
		`ALTER TABLE memberships ALTER COLUMN user_id SET NOT NULL`,
	)
}
