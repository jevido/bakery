package migrations

// M20260930000090AddAgentHeartbeatPolicy adds an Agent's Heartbeat policy
// (agents): the timer, its interval and Wake on demand, plus when its last
// timer Run was claimed.
type M20260930000090AddAgentHeartbeatPolicy struct{}

func (r *M20260930000090AddAgentHeartbeatPolicy) Signature() string {
	return "20260930000090_add_agent_heartbeat_policy"
}

func (r *M20260930000090AddAgentHeartbeatPolicy) Up() error {
	return sqls(
		`ALTER TABLE agents ADD COLUMN heartbeat_enabled boolean NOT NULL DEFAULT false`,
		`ALTER TABLE agents ADD COLUMN heartbeat_interval_sec integer NOT NULL DEFAULT 300`,
		`ALTER TABLE agents ADD COLUMN wake_on_demand boolean NOT NULL DEFAULT true`,
		`ALTER TABLE agents ADD COLUMN last_heartbeat_at timestamptz NULL`,
		`ALTER TABLE agents ADD CONSTRAINT agents_heartbeat_interval_sec CHECK (heartbeat_interval_sec BETWEEN 60 AND 86400)`,
	)
}

func (r *M20260930000090AddAgentHeartbeatPolicy) Down() error {
	return sqls(
		`ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_heartbeat_interval_sec`,
		`ALTER TABLE agents DROP COLUMN IF EXISTS last_heartbeat_at`,
		`ALTER TABLE agents DROP COLUMN IF EXISTS wake_on_demand`,
		`ALTER TABLE agents DROP COLUMN IF EXISTS heartbeat_interval_sec`,
		`ALTER TABLE agents DROP COLUMN IF EXISTS heartbeat_enabled`,
	)
}
