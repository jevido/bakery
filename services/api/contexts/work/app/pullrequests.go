package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// WorkProducts keeps the Work products of Issues.
type WorkProducts interface {
	// WorkProducts lists the Issue's Work products, newest first.
	WorkProducts(ctx context.Context, issueID uint64) ([]domain.WorkProduct, error)
	WorkProduct(ctx context.Context, issueID uint64, typ domain.WorkProductType, externalID string) (domain.WorkProduct, bool, error)
	// SaveWorkProduct stores a new one, or a known one's title, link and
	// status, and answers it as stored.
	SaveWorkProduct(ctx context.Context, w domain.WorkProduct) (domain.WorkProduct, error)
}

// OpenedPullRequest is a Pull request as the git host answered it.
type OpenedPullRequest struct {
	Provider string
	Number   int
	URL      string
	Title    string
}

// PullRequests opens a Pull request from the head branch into the
// Application's branch, or answers the one open from it
// (deployments.OpenPullRequest). Its errors are the ErrPullRequest ones
// below when it cannot.
type PullRequests func(ctx context.Context, applicationID uint64, head, title, body string) (OpenedPullRequest, error)

// What keeps The Bakery from opening a Pull request, as deployments tells.
var (
	ErrPullRequestNoToken      = errors.New("no git host token")
	ErrPullRequestUnknownHost  = errors.New("unknown git host")
	ErrPullRequestNotPushed    = errors.New("branch not pushed")
	ErrPullRequestNoRepository = errors.New("no git repository")
)

// PullRequestRefusedError is a Pull request The Bakery cannot open, with
// what the person or Agent can do about it.
type PullRequestRefusedError struct{ Message string }

func (e *PullRequestRefusedError) Error() string { return e.Message }

// PullRequestInput is what a Pull request is opened with; empty fields take
// their defaults.
type PullRequestInput struct {
	Title string
	Body  string
}

// OpenPullRequest opens the Pull request from the Issue's Agent branch
// into its Application's branch and records it as a pull_request Work
// product. An Agent may only while its Run holds the Issue's Checkout.
// created is false when the Issue already had it open; issueURL is the
// Issue page, for the default body.
func (s *Service) OpenPullRequest(ctx context.Context, guildID uint64, by domain.Actor, ref string, in PullRequestInput, issueURL func(identifier string) string, visible Visible) (w domain.WorkProduct, created bool, err error) {
	if s.PullRequests == nil || s.WorkProducts == nil {
		return domain.WorkProduct{}, false, errors.New("pull requests are not wired")
	}
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.WorkProduct{}, false, err
	}
	if by.AgentID != 0 && i.CheckoutRunID != by.RunID {
		if err := s.mayTouch(ctx, i, by); err != nil {
			return domain.WorkProduct{}, false, err
		}
		return domain.WorkProduct{}, false, domain.ErrNotHolder
	}
	if i.ApplicationID == 0 {
		return domain.WorkProduct{}, false, &PullRequestRefusedError{"The issue has no application: pick one on the issue first"}
	}
	prefix, err := s.IssuePrefix(ctx, guildID)
	if err != nil {
		return domain.WorkProduct{}, false, err
	}
	identifier := domain.Identifier(prefix, i.Number)
	head := domain.AgentBranch(identifier)
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = identifier + " " + i.Title
	}
	body := in.Body
	if strings.TrimSpace(body) == "" {
		body = fmt.Sprintf("Opened by The Bakery for [%s](%s).", identifier, issueURL(identifier))
	}
	pr, err := s.PullRequests(ctx, i.ApplicationID, head, title, body)
	switch {
	case errors.Is(err, ErrPullRequestNoToken):
		return domain.WorkProduct{}, false, &PullRequestRefusedError{"The application's webhook has no git host token"}
	case errors.Is(err, ErrPullRequestUnknownHost):
		return domain.WorkProduct{}, false, &PullRequestRefusedError{"The Bakery does not know this repository's git host yet: let the webhook receive one call first"}
	case errors.Is(err, ErrPullRequestNotPushed):
		return domain.WorkProduct{}, false, &PullRequestRefusedError{"Push the branch " + head + " first"}
	case errors.Is(err, ErrPullRequestNoRepository):
		return domain.WorkProduct{}, false, &PullRequestRefusedError{"The issue's application is not built from a git repository"}
	case err != nil:
		return domain.WorkProduct{}, false, err
	}
	w, found, err := s.WorkProducts.WorkProduct(ctx, i.ID, domain.PullRequestProduct, fmt.Sprint(pr.Number))
	if err != nil {
		return domain.WorkProduct{}, false, err
	}
	if found {
		// The git host answered it open: one recorded closed was reopened.
		if changed, _ := w.Move(domain.PullRequestOpen); changed {
			w, err = s.WorkProducts.SaveWorkProduct(ctx, w)
		}
		return w, false, err
	}
	if w, err = s.WorkProducts.SaveWorkProduct(ctx, domain.NewPullRequest(i, i.ApplicationID, pr.Provider, pr.Number, pr.Title, pr.URL, by)); err != nil {
		return domain.WorkProduct{}, false, err
	}
	s.publish(ctx, domain.PullRequestOpened{Happened: s.happened(by), Issue: i, WorkProduct: w})
	return w, true, nil
}

// IssueWorkProducts lists the Issue's Work products, newest first.
func (s *Service) IssueWorkProducts(ctx context.Context, issueID uint64) ([]domain.WorkProduct, error) {
	if s.WorkProducts == nil {
		return nil, nil
	}
	return s.WorkProducts.WorkProducts(ctx, issueID)
}
