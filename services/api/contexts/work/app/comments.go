package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Comments keeps Comments.
type Comments interface {
	// Comments lists the Issue's Comments, oldest first, deleted ones
	// included.
	Comments(ctx context.Context, issueID uint64) ([]domain.Comment, error)
	Comment(ctx context.Context, id uint64) (domain.Comment, bool, error)
	// CreateComment stores the Comment and touches its Issue's updated_at,
	// so the Issue sorts up in lists.
	CreateComment(ctx context.Context, c domain.Comment) (domain.Comment, error)
	SaveComment(ctx context.Context, c domain.Comment) (domain.Comment, error)
}

// Comments lists the thread of an Issue the person may see.
func (s *Service) Comments(ctx context.Context, guildID uint64, ref string, visible Visible) ([]domain.Comment, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return nil, err
	}
	return s.comments.Comments(ctx, i.ID)
}

// WriteComment adds the Member's Comment to the Issue.
func (s *Service) WriteComment(ctx context.Context, guildID, memberID uint64, ref, body string, visible Visible) (domain.Comment, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.Comment{}, err
	}
	c, err := domain.NewComment(i.ID, memberID, body)
	if err != nil {
		return domain.Comment{}, err
	}
	c, err = s.comments.CreateComment(ctx, c)
	if err != nil {
		return domain.Comment{}, err
	}
	s.publish(ctx, domain.CommentWritten{Happened: s.happened(memberID), Issue: i, Comment: c})
	return c, nil
}

// comment finds a Comment on the Issue; one on another Issue is
// ErrNotFound.
func (s *Service) comment(ctx context.Context, guildID uint64, ref string, commentID uint64, visible Visible) (domain.Issue, domain.Comment, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.Issue{}, domain.Comment{}, err
	}
	c, found, err := s.comments.Comment(ctx, commentID)
	if err != nil {
		return domain.Issue{}, domain.Comment{}, err
	}
	if !found || c.IssueID != i.ID {
		return domain.Issue{}, domain.Comment{}, ErrNotFound
	}
	return i, c, nil
}

// EditComment replaces the body of the Member's own Comment.
func (s *Service) EditComment(ctx context.Context, guildID, memberID uint64, ref string, commentID uint64, body string, visible Visible) (domain.Comment, error) {
	_, c, err := s.comment(ctx, guildID, ref, commentID, visible)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := c.Edit(memberID, body); err != nil {
		return domain.Comment{}, err
	}
	return s.comments.SaveComment(ctx, c)
}

// DeleteComment deletes the Member's own Comment; it stays in the thread
// as deleted.
func (s *Service) DeleteComment(ctx context.Context, guildID, memberID uint64, ref string, commentID uint64, visible Visible) error {
	i, c, err := s.comment(ctx, guildID, ref, commentID, visible)
	if err != nil {
		return err
	}
	if err := c.Delete(memberID, s.now()); err != nil {
		return err
	}
	if _, err = s.comments.SaveComment(ctx, c); err != nil {
		return err
	}
	s.publish(ctx, domain.CommentDeleted{Happened: s.happened(memberID), Issue: i, Comment: c})
	return nil
}
