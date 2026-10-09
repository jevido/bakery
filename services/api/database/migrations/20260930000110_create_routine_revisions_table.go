package migrations

// M20260930000110CreateRoutineRevisionsTable keeps each Routine's Routine
// revisions (work), the newest one on the Routine and the one each Routine
// run ran. Every Routine that exists gets revision 1 of what it holds now,
// without an author or a Change summary, in the JSON shape
// domain.SnapshotOf writes.
type M20260930000110CreateRoutineRevisionsTable struct{}

func (r *M20260930000110CreateRoutineRevisionsTable) Signature() string {
	return "20260930000110_create_routine_revisions_table"
}

func (r *M20260930000110CreateRoutineRevisionsTable) Up() error {
	return sqls(
		`CREATE TABLE routine_revisions (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			routine_id bigint NOT NULL REFERENCES routines (id) ON DELETE CASCADE,
			revision_number integer NOT NULL,
			title text NOT NULL,
			description text NOT NULL DEFAULT '',
			snapshot jsonb NOT NULL,
			change_summary text NULL,
			restored_from_revision_id bigint NULL REFERENCES routine_revisions (id) ON DELETE SET NULL,
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_by_agent_id bigint REFERENCES agents (id),
			created_at timestamptz NOT NULL,
			UNIQUE (routine_id, revision_number)
		)`,
		`ALTER TABLE routines
			ADD COLUMN latest_revision_id bigint NULL REFERENCES routine_revisions (id) ON DELETE SET NULL,
			ADD COLUMN latest_revision_number integer NOT NULL DEFAULT 0`,
		`ALTER TABLE routine_runs ADD COLUMN routine_revision_id bigint NULL REFERENCES routine_revisions (id) ON DELETE SET NULL`,
		`INSERT INTO routine_revisions (guild_id, routine_id, revision_number, title, description, snapshot, created_at)
		SELECT r.guild_id, r.id, 1, r.title, r.description, jsonb_build_object(
			'version', 1,
			'routine', jsonb_build_object(
				'id', r.id, 'guild_id', r.guild_id, 'project_id', COALESCE(r.project_id, 0), 'goal_id', COALESCE(r.goal_id, 0),
				'parent_issue_id', COALESCE(r.parent_issue_id, 0), 'assignee_agent_id', COALESCE(r.assignee_agent_id, 0),
				'title', r.title, 'description', r.description, 'priority', r.priority, 'status', r.status,
				'concurrency_policy', r.concurrency_policy, 'catch_up_policy', r.catch_up_policy,
				'variables', COALESCE((SELECT jsonb_agg(jsonb_build_object(
					'name', v->'name', 'label', COALESCE(v->'label', '""'), 'type', v->'type', 'default_value', v->'default_value',
					'required', COALESCE(v->'required', 'false'),
					'options', CASE WHEN jsonb_typeof(v->'options') = 'array' THEN v->'options' ELSE '[]'::jsonb END
				) ORDER BY n) FROM jsonb_array_elements(r.variables) WITH ORDINALITY AS e(v, n)), '[]'::jsonb)),
			'triggers', COALESCE((SELECT jsonb_agg(jsonb_build_object(
				'id', t.id, 'kind', t.kind, 'label', t.label, 'enabled', t.enabled,
				'cron_expression', t.cron_expression, 'timezone', t.timezone, 'public_id', COALESCE(t.public_id, ''),
				'signing_mode', COALESCE(t.signing_mode, ''), 'replay_window_sec', COALESCE(t.replay_window_sec, 0)
			) ORDER BY t.id) FROM routine_triggers t WHERE t.routine_id = r.id), '[]'::jsonb)
		), COALESCE(r.updated_at, r.created_at, now())
		FROM routines r`,
		`UPDATE routines r SET latest_revision_id = v.id, latest_revision_number = 1
		FROM routine_revisions v WHERE v.routine_id = r.id AND v.revision_number = 1`,
	)
}

func (r *M20260930000110CreateRoutineRevisionsTable) Down() error {
	return sqls(
		`ALTER TABLE routine_runs DROP COLUMN IF EXISTS routine_revision_id`,
		`ALTER TABLE routines DROP COLUMN IF EXISTS latest_revision_number, DROP COLUMN IF EXISTS latest_revision_id`,
		`DROP TABLE IF EXISTS routine_revisions`,
	)
}
