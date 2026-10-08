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

// WriteComment adds the Member's or Agent's Comment to the Issue. A Comment
// by the Issue's own Agent assignee does not wake it again.
func (s *Service) WriteComment(ctx context.Context, guildID uint64, by domain.Actor, ref, body string, visible Visible) (domain.Comment, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.Comment{}, err
	}
	c, err := domain.NewComment(i.ID, by, body)
	if err != nil {
		return domain.Comment{}, err
	}
	c, err = s.comments.CreateComment(ctx, c)
	if err != nil {
		return domain.Comment{}, err
	}
	s.publish(ctx, domain.CommentWritten{Happened: s.happened(by), Issue: i, Comment: c})
	if s.Commented != nil && i.AssigneeAgentID != 0 && by.AgentID != i.AssigneeAgentID && i.Status != domain.Done && i.Status != domain.IssueCancelled {
		if err := s.Commented(ctx, i, c); err != nil {
			s.Logf("work: after comment %d on issue %d: %v", c.ID, i.ID, err)
		}
	}
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

// EditComment replaces the body of one's own Comment.
func (s *Service) EditComment(ctx context.Context, guildID uint64, by domain.Actor, ref string, commentID uint64, body string, visible Visible) (domain.Comment, error) {
	_, c, err := s.comment(ctx, guildID, ref, commentID, visible)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := c.Edit(by, body); err != nil {
		return domain.Comment{}, err
	}
	return s.comments.SaveComment(ctx, c)
}

// DeleteComment deletes one's own Comment; it stays in the thread
// as deleted.
func (s *Service) DeleteComment(ctx context.Context, guildID uint64, by domain.Actor, ref string, commentID uint64, visible Visible) error {
	i, c, err := s.comment(ctx, guildID, ref, commentID, visible)
	if err != nil {
		return err
	}
	if err := c.Delete(by, s.now()); err != nil {
		return err
	}
	if _, err = s.comments.SaveComment(ctx, c); err != nil {
		return err
	}
	s.publish(ctx, domain.CommentDeleted{Happened: s.happened(by), Issue: i, Comment: c})
	return nil
}

// CommentsOfGuild returns the Guild's Comments with these ids, in that
// order, leaving out deleted ones and ids that are not the Guild's.
func (s *Service) CommentsOfGuild(ctx context.Context, guildID uint64, ids []uint64) ([]domain.Comment, error) {
	inGuild := map[uint64]bool{}
	var out []domain.Comment
	for _, id := range ids {
		c, found, err := s.comments.Comment(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found || c.DeletedAt != nil {
			continue
		}
		ok, seen := inGuild[c.IssueID]
		if !seen {
			i, found, err := s.issues.Issue(ctx, c.IssueID)
			if err != nil {
				return nil, err
			}
			ok = found && i.GuildID == guildID
			inGuild[c.IssueID] = ok
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}
