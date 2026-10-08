package migrations

// M20260930000080CreateActivityEventsTable stores the Guild's Activity
// (work). entity_id and project_id have no foreign key: an event outlives
// the Goal or Issue it is about, and the Project it was in.
type M20260930000080CreateActivityEventsTable struct{}

func (r *M20260930000080CreateActivityEventsTable) Signature() string {
	return "20260930000080_create_activity_events_table"
}

func (r *M20260930000080CreateActivityEventsTable) Up() error {
	return sqls(
		`CREATE TABLE activity_events (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			actor_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			action text NOT NULL,
			entity_type text NOT NULL,
			entity_id bigint NOT NULL,
			project_id bigint,
			details jsonb NOT NULL DEFAULT '{}',
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX activity_events_guild_id_id_index ON activity_events (guild_id, id DESC)`,
		`CREATE INDEX activity_events_entity_index ON activity_events (entity_type, entity_id, id)`,
	)
}

func (r *M20260930000080CreateActivityEventsTable) Down() error {
	return sqls(`DROP TABLE IF EXISTS activity_events`)
}
