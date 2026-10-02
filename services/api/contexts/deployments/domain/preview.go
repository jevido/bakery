package domain

import (
	"errors"
	"strconv"
	"time"
)

// PreviewState is whether a Preview's Pull request is open.
type PreviewState string

const (
	PreviewOpen   PreviewState = "open"
	PreviewClosed PreviewState = "closed"
)

var ErrPreviewClosed = errors.New("the pull request of this preview is closed")

// Preview is a copy of an Application built from one open Pull request's
// head branch. One per Application and Number.
type Preview struct {
	ID            uint64
	ApplicationID uint64
	// Number is the Pull request's number (GitLab: its iid).
	Number int
	Branch string
	Title  string
	// URL is the Pull request's web page.
	URL string
	// Provider is the git host that announced it; API is that host's REST
	// base for the repository (comments go there).
	Provider Provider
	API      string
	State    PreviewState
	// CommentID is the Preview comment Bakery keeps on the Pull request,
	// empty until the first one is posted.
	CommentID string
	CreatedAt time.Time
	ClosedAt  *time.Time
}

// Close ends an open Preview at now.
func (p *Preview) Close(now time.Time) error {
	if p.State != PreviewOpen {
		return ErrPreviewClosed
	}
	p.State, p.ClosedAt = PreviewClosed, &now
	return nil
}

// Reopen opens a closed Preview again, as when its Pull request is
// reopened.
func (p *Preview) Reopen() {
	p.State, p.ClosedAt = PreviewOpen, nil
}

// PreviewDomain is where Preview number is served: pr-<n>.<primary Domain>.
func PreviewDomain(number int, primary string) string {
	return "pr-" + strconv.Itoa(number) + "." + primary
}
