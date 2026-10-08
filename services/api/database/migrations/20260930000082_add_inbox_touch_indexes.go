package migrations

// M20260930000082AddInboxTouchIndexes lets the Inbox (work) find the Issues
// a Member is Touched by through their Comments and Activity events without
// reading every Comment and event.
type M20260930000082AddInboxTouchIndexes struct{}

func (r *M20260930000082AddInboxTouchIndexes) Signature() string {
	return "20260930000082_add_inbox_touch_indexes"
}

func (r *M20260930000082AddInboxTouchIndexes) Up() error {
	return sqls(
		`CREATE INDEX issue_comments_author_member_id_issue_id_index ON issue_comments (author_member_id, issue_id)`,
		`CREATE INDEX activity_events_actor_member_id_entity_index ON activity_events (actor_member_id, entity_type, entity_id)`,
	)
}

func (r *M20260930000082AddInboxTouchIndexes) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS activity_events_actor_member_id_entity_index`,
		`DROP INDEX IF EXISTS issue_comments_author_member_id_issue_id_index`,
	)
}
