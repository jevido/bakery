package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// ErrCommentGone is an Edit of a Preview comment someone deleted.
var ErrCommentGone = errors.New("the preview comment is gone")

// CommentTarget is the Pull request a Preview comment goes on.
type CommentTarget struct {
	Provider domain.Provider
	// API is the git host's REST base for the repository.
	API    string
	Number int
}

// PreviewComments writes comments on Pull requests through a git host's
// REST API.
type PreviewComments interface {
	// Post adds a comment and returns its id.
	Post(ctx context.Context, target CommentTarget, token, body string) (id string, err error)
	// Edit replaces a comment's body; ErrCommentGone when it was deleted.
	Edit(ctx context.Context, target CommentTarget, token, id, body string) error
}

// Commenter keeps the one Preview comment of each Preview up to date.
type Commenter struct {
	webhooks WebhookStore
	previews PreviewStore
	comments PreviewComments
	// URL is where a Domain on a Server is reached.
	URL func(domain string, serverID uint64) string
	now func() time.Time
}

func NewCommenter(webhooks WebhookStore, previews PreviewStore, comments PreviewComments) *Commenter {
	return &Commenter{webhooks: webhooks, previews: previews, comments: comments,
		URL: func(d string, _ uint64) string { return "https://" + d }, now: time.Now}
}

// Comment writes body as the Preview's comment: posted the first time,
// edited afterwards (posted again when someone deleted it). Without a Git
// host token it does nothing and reports false.
func (c *Commenter) Comment(ctx context.Context, applicationID uint64, number int, body string) (bool, error) {
	hook, found, err := c.webhooks.ByApplication(ctx, applicationID)
	if err != nil || !found || hook.GitHostToken == "" {
		return false, err
	}
	p, found, err := c.previews.ByNumber(ctx, applicationID, number)
	if err != nil || !found {
		return false, err
	}
	if p.API == "" {
		return false, errors.New("the pull request event named no API to comment through")
	}
	target := CommentTarget{Provider: p.Provider, API: p.API, Number: p.Number}
	if p.CommentID != "" {
		err := c.comments.Edit(ctx, target, hook.GitHostToken, p.CommentID, body)
		if !errors.Is(err, ErrCommentGone) {
			return err == nil, err
		}
	}
	id, err := c.comments.Post(ctx, target, hook.GitHostToken, body)
	if err != nil {
		return false, err
	}
	p.CommentID = id
	_, err = c.previews.Save(ctx, p)
	return true, err
}

// DeployedBody is the Preview comment after a Preview Deployment finished
// or failed.
func (c *Commenter) DeployedBody(d domain.Deployment, previewDomain string) string {
	var b strings.Builder
	b.WriteString("**Bakery preview**\n\n")
	commit := shortSHA(d.CommitSHA)
	if d.CommitMessage != "" {
		commit += " " + d.CommitMessage
	}
	if d.Status == domain.Finished {
		fmt.Fprintf(&b, "✅ Deployed: %s\n\n", c.URL(previewDomain, d.ServerID))
	} else {
		fmt.Fprintf(&b, "❌ Deployment failed: %s\n\n", d.Error)
	}
	fmt.Fprintf(&b, "Commit: `%s` · deployment %d · %s\n", commit, d.ID, c.now().UTC().Format("2006-01-02 15:04 UTC"))
	return b.String()
}

// RemovedBody is the Preview comment once its Pull request closed.
func (c *Commenter) RemovedBody() string {
	return "**Bakery preview**\n\n🗑️ Removed: the pull request was closed. · " + c.now().UTC().Format("2006-01-02 15:04 UTC") + "\n"
}
