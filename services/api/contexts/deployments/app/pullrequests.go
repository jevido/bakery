package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

var (
	// ErrNoGitHostToken is an Application whose Webhook has no Git host
	// token, or that has no Webhook at all.
	ErrNoGitHostToken = errors.New("the application's webhook has no git host token")
	// ErrBranchNotPushed is a head branch the git host does not have.
	ErrBranchNotPushed = errors.New("the branch is not on the git host")
	// ErrNoRepository is an Application that is not built from git.
	ErrNoRepository = errors.New("the application has no git repository")
)

// OpenedPullRequest is a Pull request on the git host. Created is false
// when it was already open.
type OpenedPullRequest struct {
	Provider domain.Provider
	Number   int
	URL      string
	Title    string
	Created  bool
}

// PullRequestHosts talks to a git host's REST API about Pull requests.
type PullRequestHosts interface {
	// FindPullRequest answers the open Pull request from head into base;
	// found is false when there is none.
	FindPullRequest(ctx context.Context, r domain.Repository, token, head, base string) (OpenedPullRequest, bool, error)
	// BranchExists tells whether the repository has the branch.
	BranchExists(ctx context.Context, r domain.Repository, token, branch string) (bool, error)
	CreatePullRequest(ctx context.Context, r domain.Repository, token, head, base, title, body string) (OpenedPullRequest, error)
}

// OpenPullRequest opens a Pull request from head into the Application's
// branch with its Webhook's Git host token, or answers the one already
// open for head.
func (w *Webhooks) OpenPullRequest(ctx context.Context, applicationID uint64, head, title, body string) (OpenedPullRequest, error) {
	a, err := w.service.applications(ctx, applicationID)
	if err != nil {
		return OpenedPullRequest{}, err
	}
	if a.BuildPack == BuildPackDockerImage || a.GitURL == "" {
		return OpenedPullRequest{}, ErrNoRepository
	}
	hook, found, err := w.store.ByApplication(ctx, applicationID)
	if err != nil {
		return OpenedPullRequest{}, err
	}
	if !found || hook.GitHostToken == "" {
		return OpenedPullRequest{}, ErrNoGitHostToken
	}
	repo, err := hook.Repository(a.GitURL)
	if err != nil {
		return OpenedPullRequest{}, err
	}
	pr, found, err := w.Hosts.FindPullRequest(ctx, repo, hook.GitHostToken, head, a.GitBranch)
	if err != nil || found {
		return pr, err
	}
	pr, err = w.Hosts.CreatePullRequest(ctx, repo, hook.GitHostToken, head, a.GitBranch, title, body)
	if err == nil {
		pr.Created = true
		return pr, nil
	}
	// The git hosts refuse a missing head branch each in their own words,
	// so ask for the branch only once creating failed: some list a branch
	// a moment after its push landed, while creating already works.
	if pushed, berr := w.Hosts.BranchExists(ctx, repo, hook.GitHostToken, head); berr == nil && !pushed {
		return OpenedPullRequest{}, ErrBranchNotPushed
	}
	return OpenedPullRequest{}, err
}
