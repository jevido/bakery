package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Inbox keeps each Member's Read marks and Inbox archives. Saving one the
// Member already has replaces its time; deleting one they lack is no error.
type Inbox interface {
	SaveReadMark(ctx context.Context, m domain.ReadMark) error
	DeleteReadMark(ctx context.Context, issueID, memberID uint64) error
	SaveInboxArchive(ctx context.Context, a domain.InboxArchive) error
	DeleteInboxArchive(ctx context.Context, issueID, memberID uint64) error
}

// MarkRead sets the Member's Read mark on the Issue to now. Like the other
// Inbox commands it changes nothing about the Issue and publishes no event.
func (s *Service) MarkRead(ctx context.Context, guildID, memberID uint64, ref string, visible Visible) (domain.ReadMark, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.ReadMark{}, err
	}
	m := domain.ReadMark{IssueID: i.ID, MemberID: memberID, At: s.now()}
	return m, s.inbox.SaveReadMark(ctx, m)
}

// MarkUnread removes the Member's Read mark from the Issue.
func (s *Service) MarkUnread(ctx context.Context, guildID, memberID uint64, ref string, visible Visible) error {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return err
	}
	return s.inbox.DeleteReadMark(ctx, i.ID, memberID)
}

// ArchiveFromInbox sets the Member's Inbox archive on the Issue to now.
func (s *Service) ArchiveFromInbox(ctx context.Context, guildID, memberID uint64, ref string, visible Visible) (domain.InboxArchive, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.InboxArchive{}, err
	}
	a := domain.InboxArchive{IssueID: i.ID, MemberID: memberID, At: s.now()}
	return a, s.inbox.SaveInboxArchive(ctx, a)
}

// UnarchiveFromInbox removes the Member's Inbox archive from the Issue.
func (s *Service) UnarchiveFromInbox(ctx context.Context, guildID, memberID uint64, ref string, visible Visible) error {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return err
	}
	return s.inbox.DeleteInboxArchive(ctx, i.ID, memberID)
}
