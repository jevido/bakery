package migrations

// M20260930000112AddIssueConversations lets an Issue be a Conversation
// (work): one Member's chat with one Agent, at most one per Guild, Agent
// and Member, with its Conversation state and Session boundary. A
// Comment keeps the Run an Agent wrote it in; runs are agents' table, so
// run_id has no foreign key.
type M20260930000112AddIssueConversations struct{}

func (r *M20260930000112AddIssueConversations) Signature() string {
	return "20260930000112_add_issue_conversations"
}

func (r *M20260930000112AddIssueConversations) Up() error {
	return sqls(
		`ALTER TABLE issues
			ADD COLUMN conversation_agent_id bigint NULL,
			ADD COLUMN conversation_member_id bigint NULL,
			ADD COLUMN conversation_state text NULL,
			ADD COLUMN conversation_boundary_comment_id bigint NULL REFERENCES issue_comments (id) ON DELETE SET NULL,
			ADD CONSTRAINT issues_conversation_check CHECK (
				(conversation_agent_id IS NULL AND conversation_member_id IS NULL AND conversation_state IS NULL
					AND conversation_boundary_comment_id IS NULL)
				OR (conversation_agent_id IS NOT NULL AND conversation_member_id IS NOT NULL
					AND conversation_state IN ('active', 'waiting') AND conversation_agent_id = assignee_agent_id))`,
		`CREATE UNIQUE INDEX issues_conversation_unique ON issues (guild_id, conversation_agent_id, conversation_member_id)
			WHERE conversation_agent_id IS NOT NULL`,
		`ALTER TABLE issue_comments ADD COLUMN run_id bigint NULL`,
	)
}

func (r *M20260930000112AddIssueConversations) Down() error {
	return sqls(
		`ALTER TABLE issue_comments DROP COLUMN IF EXISTS run_id`,
		`DROP INDEX IF EXISTS issues_conversation_unique`,
		`ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_conversation_check,
			DROP COLUMN IF EXISTS conversation_boundary_comment_id,
			DROP COLUMN IF EXISTS conversation_state,
			DROP COLUMN IF EXISTS conversation_member_id,
			DROP COLUMN IF EXISTS conversation_agent_id`,
	)
}
