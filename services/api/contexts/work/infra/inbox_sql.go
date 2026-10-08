package infra

import (
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// sqlFragment is a condition or an expression on the issues table, with
// the arguments for its placeholders. The Inbox's rules are written here
// once, for the Issues list, the Inbox badge's count and the per-Issue
// Inbox states; they follow Paperclip's (server/src/services/issues.ts).
type sqlFragment struct {
	sql  string
	args []any
}

// touched holds for the Issues the Member is Touched by.
func touched(m uint64) sqlFragment {
	return sqlFragment{`(issues.created_by_member_id = ? OR issues.assignee_member_id = ?
		OR EXISTS (SELECT 1 FROM issue_comments c WHERE c.issue_id = issues.id AND c.author_member_id = ?)
		OR EXISTS (SELECT 1 FROM activity_events e WHERE e.entity_type = 'issue' AND e.entity_id = issues.id AND e.actor_member_id = ?))`,
		[]any{m, m, m, m}}
}

// lastTouch is the Member's Last touch of the Issue, the epoch when they
// have none. For an Assignee the Issue's last update is its latest
// issue.created or issue.updated event, not updated_at: a Comment moves
// updated_at, and with it no Assignee would ever see the Issue Unread.
func lastTouch(m uint64) sqlFragment {
	return sqlFragment{`GREATEST(
		COALESCE((SELECT max(c.created_at) FROM issue_comments c WHERE c.issue_id = issues.id AND c.author_member_id = ?), 'epoch'::timestamptz),
		COALESCE((SELECT r.last_read_at FROM issue_read_states r WHERE r.issue_id = issues.id AND r.member_id = ?), 'epoch'::timestamptz),
		CASE WHEN issues.created_by_member_id = ? THEN issues.created_at ELSE 'epoch'::timestamptz END,
		CASE WHEN issues.assignee_member_id = ? THEN COALESCE((SELECT max(e.created_at) FROM activity_events e
			WHERE e.entity_type = 'issue' AND e.entity_id = issues.id AND e.action IN (?, ?)), issues.created_at) ELSE 'epoch'::timestamptz END)`,
		[]any{m, m, m, m, domain.IssueCreatedAction, domain.IssueUpdatedAction}}
}

// unread holds for the Touched Issues with a Comment, not deleted, by
// someone else after the Member's Last touch.
func unread(m uint64) sqlFragment {
	t, lt := touched(m), lastTouch(m)
	args := append(append([]any{}, t.args...), m)
	return sqlFragment{`(` + t.sql + ` AND EXISTS (SELECT 1 FROM issue_comments c WHERE c.issue_id = issues.id
		AND c.deleted_at IS NULL AND (c.author_member_id IS NULL OR c.author_member_id <> ?) AND c.created_at > ` + lt.sql + `))`,
		append(args, lt.args...)}
}

// inMine holds for the Touched Issues in the Member's Mine tab: not in
// their Inbox archive, or Resurfaced since it.
func inMine(m uint64) sqlFragment {
	statuses := make([]string, len(domain.ResurfaceStatuses))
	for n, s := range domain.ResurfaceStatuses {
		statuses[n] = string(s)
	}
	t := touched(m)
	// jsonb_extract_path_text rather than ->: the ORM mangles -> 'key'.
	to := []any{"changes", "status", "to"}
	args := append(append([]any{}, t.args...), m, m, domain.IssueUpdatedAction)
	args = append(append(append(append(args, to...), statuses), to...), string(domain.Done))
	return sqlFragment{`(` + t.sql + ` AND NOT EXISTS (SELECT 1 FROM issue_inbox_archives a
		WHERE a.issue_id = issues.id AND a.member_id = ?
		AND NOT (
			EXISTS (SELECT 1 FROM issue_comments c WHERE c.issue_id = issues.id AND c.deleted_at IS NULL
				AND c.author_member_id IS NOT NULL AND c.author_member_id <> ? AND c.created_at > a.archived_at)
			OR EXISTS (SELECT 1 FROM activity_events e WHERE e.entity_type = 'issue' AND e.entity_id = issues.id
				AND e.action = ? AND e.created_at > a.archived_at
				AND jsonb_extract_path_text(e.details, ?, ?, ?) IN ?
				AND NOT (jsonb_extract_path_text(e.details, ?, ?, ?) = ? AND issues.completed_at IS NOT NULL AND a.archived_at >= issues.completed_at)))))`,
		args}
}
