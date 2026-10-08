package infra

import (
	"context"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Inbox keeps each Member's Read marks and Inbox archives, one row of each
// per (Issue, Member).
type Inbox struct{}

func (Inbox) SaveReadMark(ctx context.Context, m domain.ReadMark) error {
	_, err := facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO issue_read_states (issue_id, member_id, last_read_at) VALUES (?, ?, ?)
		ON CONFLICT (issue_id, member_id) DO UPDATE SET last_read_at = EXCLUDED.last_read_at`,
		m.IssueID, m.MemberID, m.At)
	return err
}

func (Inbox) DeleteReadMark(ctx context.Context, issueID, memberID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Exec(
		`DELETE FROM issue_read_states WHERE issue_id = ? AND member_id = ?`, issueID, memberID)
	return err
}

func (Inbox) SaveInboxArchive(ctx context.Context, a domain.InboxArchive) error {
	_, err := facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO issue_inbox_archives (issue_id, member_id, archived_at) VALUES (?, ?, ?)
		ON CONFLICT (issue_id, member_id) DO UPDATE SET archived_at = EXCLUDED.archived_at`,
		a.IssueID, a.MemberID, a.At)
	return err
}

func (Inbox) DeleteInboxArchive(ctx context.Context, issueID, memberID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Exec(
		`DELETE FROM issue_inbox_archives WHERE issue_id = ? AND member_id = ?`, issueID, memberID)
	return err
}
