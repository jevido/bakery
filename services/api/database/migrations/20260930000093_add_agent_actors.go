package migrations

// M20260930000093AddAgentActors lets an Agent be the Actor or author of
// what a Member could do in work: an Activity event, an Issue's creator, a
// Comment, an Issue document and its Revisions, an Approval's Requester and
// an Approval comment. Each agent column sits beside its member column,
// never both set.
type M20260930000093AddAgentActors struct{}

func (r *M20260930000093AddAgentActors) Signature() string {
	return "20260930000093_add_agent_actors"
}

// agentActorColumns is each table's member column and the agent column
// beside it.
var agentActorColumns = []struct{ table, member, agent string }{
	{"activity_events", "actor_member_id", "actor_agent_id"},
	{"issues", "created_by_member_id", "created_by_agent_id"},
	{"issue_comments", "author_member_id", "author_agent_id"},
	{"issue_documents", "created_by_member_id", "created_by_agent_id"},
	{"issue_documents", "updated_by_member_id", "updated_by_agent_id"},
	{"issue_document_revisions", "created_by_member_id", "created_by_agent_id"},
	{"approvals", "requested_by_member_id", "requested_by_agent_id"},
	{"approval_comments", "author_member_id", "author_agent_id"},
}

func (r *M20260930000093AddAgentActors) Up() error {
	var stmts []string
	for _, c := range agentActorColumns {
		stmts = append(stmts,
			`ALTER TABLE `+c.table+` ADD COLUMN `+c.agent+` bigint REFERENCES agents (id) ON DELETE SET NULL`,
			`ALTER TABLE `+c.table+` ADD CONSTRAINT `+c.table+`_one_`+c.agent+` CHECK (`+c.member+` IS NULL OR `+c.agent+` IS NULL)`,
		)
	}
	return sqls(stmts...)
}

func (r *M20260930000093AddAgentActors) Down() error {
	var stmts []string
	for _, c := range agentActorColumns {
		stmts = append(stmts, `ALTER TABLE `+c.table+` DROP COLUMN IF EXISTS `+c.agent)
	}
	return sqls(stmts...)
}
