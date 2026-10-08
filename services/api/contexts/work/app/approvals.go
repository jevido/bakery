package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Approvals keeps Approvals with their Linked issues.
type Approvals interface {
	// CreateApproval stores the Approval and its Linked issues together.
	CreateApproval(ctx context.Context, a domain.Approval) (domain.Approval, error)
	Approval(ctx context.Context, id uint64) (domain.Approval, bool, error)
	// Approvals lists the Guild's Approvals in these statuses (any for
	// none), newest first.
	Approvals(ctx context.Context, guildID uint64, statuses []domain.ApprovalStatus) ([]domain.Approval, error)
	// IssueApprovals lists the Approvals linked to the Issue, newest first.
	IssueApprovals(ctx context.Context, issueID uint64) ([]domain.Approval, error)
	// SaveApproval writes the Approval only while its stored status is one
	// of from; saved is false when someone else moved it first.
	SaveApproval(ctx context.Context, a domain.Approval, from []domain.ApprovalStatus) (saved bool, err error)
	// ApprovalComments lists the Approval's comments, oldest first.
	ApprovalComments(ctx context.Context, approvalID uint64) ([]domain.ApprovalComment, error)
	CreateApprovalComment(ctx context.Context, c domain.ApprovalComment) (domain.ApprovalComment, error)
}

// Actionable is the status filter for every Approval still waiting for a
// Decision.
const Actionable = "actionable"

// ApprovalStatuses reads a status filter: a comma list of Approval
// statuses and "actionable"; empty is every status.
func ApprovalStatuses(filter string) ([]domain.ApprovalStatus, error) {
	var out []domain.ApprovalStatus
	for _, s := range strings.Split(filter, ",") {
		s = strings.TrimSpace(s)
		switch s {
		case "":
			continue
		case Actionable:
			out = append(out, domain.StatusPending, domain.StatusRevisionRequested)
			continue
		}
		st, err := domain.ParseApprovalStatus(s)
		if err != nil {
			return nil, &domain.FieldError{Field: "status", Message: "status must be pending, revision_requested, approved, rejected or actionable"}
		}
		out = append(out, st)
	}
	return out, nil
}

// RequestApproval asks the Board of the Guild, as the Member, to decide on
// the payload, about the Issues; each must be one of the Guild's that the
// Member may view.
func (s *Service) RequestApproval(ctx context.Context, guildID, memberID uint64, typ string, p domain.BoardApprovalPayload, issueIDs []uint64, visible Visible) (domain.Approval, error) {
	t, err := domain.ParseApprovalType(typ)
	if err != nil {
		return domain.Approval{}, err
	}
	a, err := domain.RequestApproval(guildID, memberID, t, p, issueIDs)
	if err != nil {
		return domain.Approval{}, err
	}
	found, err := s.VisibleIssues(ctx, a.IssueIDs, visible)
	if err != nil {
		return domain.Approval{}, err
	}
	ours := map[uint64]bool{}
	for _, i := range found {
		ours[i.ID] = i.GuildID == guildID
	}
	for _, id := range a.IssueIDs {
		if !ours[id] {
			return domain.Approval{}, &domain.FieldError{Field: "issue_ids", Message: "issue " + strconv.FormatUint(id, 10) + " not found"}
		}
	}
	a.CreatedAt = s.now()
	a.UpdatedAt = a.CreatedAt
	a, err = s.approvals.CreateApproval(ctx, a)
	if err != nil {
		return domain.Approval{}, err
	}
	s.publish(ctx, domain.ApprovalRequested{Happened: s.happened(memberID), Approval: a})
	return a, nil
}

// Approvals lists the Guild's Approvals in a status filter (see
// ApprovalStatuses), newest first.
func (s *Service) Approvals(ctx context.Context, guildID uint64, status string) ([]domain.Approval, error) {
	sts, err := ApprovalStatuses(status)
	if err != nil {
		return nil, err
	}
	return s.approvals.Approvals(ctx, guildID, sts)
}

// ApprovalInGuild reports whether the Approval exists and belongs to the
// Guild.
func (s *Service) ApprovalInGuild(ctx context.Context, id, guildID uint64) (bool, error) {
	a, found, err := s.approvals.Approval(ctx, id)
	return found && a.GuildID == guildID, err
}

// Approval finds one of the Guild's Approvals; another Guild's is
// ErrNotFound.
func (s *Service) Approval(ctx context.Context, guildID, id uint64) (domain.Approval, error) {
	a, found, err := s.approvals.Approval(ctx, id)
	if err != nil {
		return domain.Approval{}, err
	}
	if !found || a.GuildID != guildID {
		return domain.Approval{}, ErrNotFound
	}
	return a, nil
}

// ApprovalIssues is the Approval's Linked issues the person may view, in
// the order they were linked.
func (s *Service) ApprovalIssues(ctx context.Context, guildID, id uint64, visible Visible) ([]domain.Issue, error) {
	a, err := s.Approval(ctx, guildID, id)
	if err != nil {
		return nil, err
	}
	found, err := s.VisibleIssues(ctx, a.IssueIDs, visible)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]domain.Issue, len(found))
	for _, i := range found {
		byID[i.ID] = i
	}
	out := make([]domain.Issue, 0, len(found))
	for _, id := range a.IssueIDs {
		if i, ok := byID[id]; ok && i.GuildID == guildID {
			out = append(out, i)
		}
	}
	return out, nil
}

// IssueApprovals is the Approvals linked to the Issue, newest first; an
// Issue that is another Guild's or that the person may not view is
// ErrNotFound.
func (s *Service) IssueApprovals(ctx context.Context, guildID uint64, ref string, visible Visible) ([]domain.Approval, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return nil, err
	}
	return s.approvals.IssueApprovals(ctx, i.ID)
}

// move is one change to an Approval: the statuses it may start from, the
// change itself (changed is false for the same Decision made again) and
// its event.
type move struct {
	from  []domain.ApprovalStatus
	apply func(a *domain.Approval, at time.Time) (changed bool, err error)
	event func(h domain.Happened, a domain.Approval) domain.Event
}

// moveApproval applies the move to one of the Guild's Approvals and saves
// it guarded by its from statuses, as Paperclip's resolveApproval: when
// someone else moved it first, the move is applied again to what they
// left, which answers it unchanged or refuses it.
func (s *Service) moveApproval(ctx context.Context, guildID, memberID, id uint64, m move) (domain.Approval, error) {
	for range 3 {
		a, err := s.Approval(ctx, guildID, id)
		if err != nil {
			return domain.Approval{}, err
		}
		changed, err := m.apply(&a, s.now())
		if err != nil || !changed {
			return a, err
		}
		saved, err := s.approvals.SaveApproval(ctx, a, m.from)
		if err != nil {
			return domain.Approval{}, err
		}
		if saved {
			s.publish(ctx, m.event(s.happened(memberID), a))
			return a, nil
		}
	}
	return domain.Approval{}, errors.New("approval kept changing while it was being decided")
}

var actionable = []domain.ApprovalStatus{domain.StatusPending, domain.StatusRevisionRequested}

// ApproveApproval is the Member's yes on one of the Guild's Approvals.
func (s *Service) ApproveApproval(ctx context.Context, guildID, memberID, id uint64, note string) (domain.Approval, error) {
	return s.moveApproval(ctx, guildID, memberID, id, move{
		from:  actionable,
		apply: func(a *domain.Approval, at time.Time) (bool, error) { return a.Approve(memberID, note, at) },
		event: func(h domain.Happened, a domain.Approval) domain.Event {
			return domain.ApprovalApproved{Happened: h, Approval: a}
		},
	})
}

// RejectApproval is the Member's no on one of the Guild's Approvals.
func (s *Service) RejectApproval(ctx context.Context, guildID, memberID, id uint64, note string) (domain.Approval, error) {
	return s.moveApproval(ctx, guildID, memberID, id, move{
		from:  actionable,
		apply: func(a *domain.Approval, at time.Time) (bool, error) { return a.Reject(memberID, note, at) },
		event: func(h domain.Happened, a domain.Approval) domain.Event {
			return domain.ApprovalRejected{Happened: h, Approval: a}
		},
	})
}

// RequestApprovalRevision sends a pending Approval back to its Requester.
func (s *Service) RequestApprovalRevision(ctx context.Context, guildID, memberID, id uint64, note string) (domain.Approval, error) {
	return s.moveApproval(ctx, guildID, memberID, id, move{
		from: []domain.ApprovalStatus{domain.StatusPending},
		apply: func(a *domain.Approval, at time.Time) (bool, error) {
			return true, a.RequestRevision(memberID, note, at)
		},
		event: func(h domain.Happened, a domain.Approval) domain.Event {
			return domain.RevisionRequested{Happened: h, Approval: a}
		},
	})
}

// ResubmitApproval makes the Member's own revision_requested Approval
// pending again, with a new payload when p is not nil.
func (s *Service) ResubmitApproval(ctx context.Context, guildID, memberID, id uint64, p *domain.BoardApprovalPayload) (domain.Approval, error) {
	return s.moveApproval(ctx, guildID, memberID, id, move{
		from: []domain.ApprovalStatus{domain.StatusRevisionRequested},
		apply: func(a *domain.Approval, at time.Time) (bool, error) {
			return true, a.Resubmit(memberID, p, at)
		},
		event: func(h domain.Happened, a domain.Approval) domain.Event {
			return domain.ApprovalResubmitted{Happened: h, Approval: a}
		},
	})
}

// ApprovalComments is the comment thread of one of the Guild's Approvals,
// oldest first.
func (s *Service) ApprovalComments(ctx context.Context, guildID, id uint64) ([]domain.ApprovalComment, error) {
	if _, err := s.Approval(ctx, guildID, id); err != nil {
		return nil, err
	}
	return s.approvals.ApprovalComments(ctx, id)
}

// AddApprovalComment adds the Member's comment to one of the Guild's
// Approvals.
func (s *Service) AddApprovalComment(ctx context.Context, guildID, memberID, id uint64, body string) (domain.ApprovalComment, error) {
	a, err := s.Approval(ctx, guildID, id)
	if err != nil {
		return domain.ApprovalComment{}, err
	}
	c, err := domain.NewApprovalComment(a.ID, memberID, body)
	if err != nil {
		return domain.ApprovalComment{}, err
	}
	c.CreatedAt = s.now()
	c, err = s.approvals.CreateApprovalComment(ctx, c)
	if err != nil {
		return domain.ApprovalComment{}, err
	}
	s.publish(ctx, domain.ApprovalCommentWritten{Happened: s.happened(memberID), Approval: a, Comment: c})
	return c, nil
}
