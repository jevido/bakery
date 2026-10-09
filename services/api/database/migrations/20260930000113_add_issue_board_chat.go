package migrations

// M20260930000113AddIssueBoardChat lets an Issue be its Guild's Board
// chat (work): a Conversation of the whole Board with the Guild's CEO,
// with no Conversation agent, owner or Assignee, at most one per Guild.
type M20260930000113AddIssueBoardChat struct{}

func (r *M20260930000113AddIssueBoardChat) Signature() string {
	return "20260930000113_add_issue_board_chat"
}

func (r *M20260930000113AddIssueBoardChat) Up() error {
	return sqls(
		`ALTER TABLE issues ADD COLUMN conversation_board boolean NOT NULL DEFAULT false`,
		`ALTER TABLE issues DROP CONSTRAINT issues_conversation_check,
			ADD CONSTRAINT issues_conversation_check CHECK (
				(NOT conversation_board AND conversation_agent_id IS NULL AND conversation_member_id IS NULL
					AND conversation_state IS NULL AND conversation_boundary_comment_id IS NULL)
				OR (NOT conversation_board AND conversation_agent_id IS NOT NULL AND conversation_member_id IS NOT NULL
					AND conversation_state IN ('active', 'waiting') AND conversation_agent_id = assignee_agent_id)
				OR (conversation_board AND conversation_agent_id IS NULL AND conversation_member_id IS NULL
					AND conversation_state IN ('active', 'waiting')
					AND assignee_agent_id IS NULL AND assignee_member_id IS NULL))`,
		`CREATE UNIQUE INDEX issues_board_chat_unique ON issues (guild_id) WHERE conversation_board`,
	)
}

func (r *M20260930000113AddIssueBoardChat) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS issues_board_chat_unique`,
		`DELETE FROM issues WHERE conversation_board`,
		`ALTER TABLE issues DROP CONSTRAINT IF EXISTS issues_conversation_check,
			DROP COLUMN IF EXISTS conversation_board,
			ADD CONSTRAINT issues_conversation_check CHECK (
				(conversation_agent_id IS NULL AND conversation_member_id IS NULL AND conversation_state IS NULL
					AND conversation_boundary_comment_id IS NULL)
				OR (conversation_agent_id IS NOT NULL AND conversation_member_id IS NOT NULL
					AND conversation_state IN ('active', 'waiting') AND conversation_agent_id = assignee_agent_id))`,
	)
}
