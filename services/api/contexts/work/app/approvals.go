package app

import (
	"context"
	"strconv"
	"strings"

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
			out = append(out, domain.ApprovalPending, domain.RevisionRequested)
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
