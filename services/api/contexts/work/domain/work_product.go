package domain

import (
	"errors"
	"fmt"
	"time"
)

// WorkProductType is what a Work product is.
type WorkProductType string

const (
	PullRequestProduct WorkProductType = "pull_request"
	PreviewURLProduct  WorkProductType = "preview_url"
)

// WorkProductStatus is where a Work product stands: a pull_request is
// open, merged or closed; a preview_url deploying, ready, failed or
// removed.
type WorkProductStatus string

const (
	PullRequestOpen   WorkProductStatus = "open"
	PullRequestMerged WorkProductStatus = "merged"
	PullRequestClosed WorkProductStatus = "closed"
	PreviewDeploying  WorkProductStatus = "deploying"
	PreviewReady      WorkProductStatus = "ready"
	PreviewFailed     WorkProductStatus = "failed"
	PreviewRemoved    WorkProductStatus = "removed"
)

// ErrWorkProductMove is a status a Work product cannot move to.
var ErrWorkProductMove = errors.New("the work product cannot move to that status")

// WorkProduct is something an Issue produced outside The Bakery's own
// records: a Pull request on the git host, or the link of its Preview.
// ExternalID is the Pull request's number (GitLab: its iid) for both.
type WorkProduct struct {
	ID            uint64
	GuildID       uint64
	IssueID       uint64
	ApplicationID uint64
	Type          WorkProductType
	// Provider is the git host's wire key, e.g. "forgejo".
	Provider   string
	ExternalID string
	Title      string
	URL        string
	Status     WorkProductStatus
	// CreatedBy is the Agent (with the Run it acted in) or the Member who
	// made it; nobody when The Bakery recorded one opened by hand.
	CreatedBy Actor
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewPullRequest is an open Pull request of the Issue, made by by.
func NewPullRequest(i Issue, applicationID uint64, provider string, number int, title, url string, by Actor) WorkProduct {
	return WorkProduct{
		GuildID: i.GuildID, IssueID: i.ID, ApplicationID: applicationID, Type: PullRequestProduct,
		Provider: provider, ExternalID: fmt.Sprint(number), Title: title, URL: url, Status: PullRequestOpen, CreatedBy: by,
	}
}

// NewPreviewURL is the Preview of the Issue's Pull request pr, with its
// link (empty while The Bakery does not know it yet).
func NewPreviewURL(pr WorkProduct, url string, status WorkProductStatus) WorkProduct {
	return WorkProduct{
		GuildID: pr.GuildID, IssueID: pr.IssueID, ApplicationID: pr.ApplicationID, Type: PreviewURLProduct,
		Provider: pr.Provider, ExternalID: pr.ExternalID, Title: "Preview of #" + pr.ExternalID, URL: url, Status: status,
	}
}

// Move changes the status, as the type allows: a Pull request goes from
// open to merged or closed and from closed back to open, and a merged one
// never changes again; a Preview's link may take any state after
// deploying, ready or failed, and after removed only deploying again.
// changed is false when it already had the status.
func (w *WorkProduct) Move(to WorkProductStatus) (changed bool, err error) {
	if w.Status == to {
		return false, nil
	}
	var ok bool
	switch w.Type {
	case PullRequestProduct:
		ok = (w.Status == PullRequestOpen && (to == PullRequestMerged || to == PullRequestClosed)) ||
			(w.Status == PullRequestClosed && to == PullRequestOpen)
	case PreviewURLProduct:
		switch to {
		case PreviewDeploying, PreviewReady, PreviewFailed, PreviewRemoved:
			ok = w.Status != PreviewRemoved || to == PreviewDeploying
		}
	}
	if !ok {
		return false, ErrWorkProductMove
	}
	w.Status = to
	return true, nil
}
