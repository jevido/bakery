package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// GitHosts writes Preview comments and opens Pull requests through the
// REST APIs of Forgejo and Gitea, GitHub and GitLab. A comment's API base
// comes from the Pull request event, a new Pull request's from the
// Application's git URL.
type GitHosts struct {
	Client *http.Client
}

func (g GitHosts) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// endpoints returns where a comment is posted and where one is edited, and
// the method that edits.
func endpoints(t app.CommentTarget, id string) (post, edit, editMethod string) {
	n := strconv.Itoa(t.Number)
	if t.Provider == domain.GitLab {
		return t.API + "/merge_requests/" + n + "/notes", t.API + "/merge_requests/" + n + "/notes/" + id, http.MethodPut
	}
	return t.API + "/issues/" + n + "/comments", t.API + "/issues/comments/" + id, http.MethodPatch
}

func (g GitHosts) call(ctx context.Context, t app.CommentTarget, method, url, token, body string) ([]byte, error) {
	status, out, err := g.do(ctx, t.Provider, method, url, token, map[string]string{"body": body})
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound && method != http.MethodPost {
		return nil, app.ErrCommentGone
	}
	if status/100 != 2 {
		return nil, answered(t.Provider, status, out)
	}
	return out, nil
}

// do sends one REST call with the Provider's auth headers and answers its
// status and (up to 64 KiB of) body. payload nil sends no body.
func (g GitHosts) do(ctx context.Context, p domain.Provider, method, url, token string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch p {
	case domain.GitLab:
		req.Header.Set("PRIVATE-TOKEN", token)
	case domain.GitHub:
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
	default:
		req.Header.Set("Authorization", "token "+token)
	}
	res, err := g.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	return res.StatusCode, out, nil
}

func answered(p domain.Provider, status int, out []byte) error {
	msg := strings.TrimSpace(string(out))
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return fmt.Errorf("%s answered %d: %s", p, status, msg)
}

func (g GitHosts) Post(ctx context.Context, t app.CommentTarget, token, body string) (string, error) {
	url, _, _ := endpoints(t, "")
	out, err := g.call(ctx, t, http.MethodPost, url, token, body)
	if err != nil {
		return "", err
	}
	var created struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(out, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("%s answered without a comment id", t.Provider)
	}
	return created.ID.String(), nil
}

func (g GitHosts) Edit(ctx context.Context, t app.CommentTarget, token, id, body string) error {
	_, url, method := endpoints(t, id)
	_, err := g.call(ctx, t, method, url, token, body)
	return err
}

// pullRequest is a Pull request (GitLab: merge request) as the git hosts
// answer one; GitLab names number and link iid and web_url.
type pullRequest struct {
	Number int    `json:"number"`
	IID    int    `json:"iid"`
	URL    string `json:"html_url"`
	WebURL string `json:"web_url"`
	Title  string `json:"title"`
	Head   struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (p pullRequest) opened(provider domain.Provider) app.OpenedPullRequest {
	if provider == domain.GitLab {
		return app.OpenedPullRequest{Provider: provider, Number: p.IID, URL: p.WebURL, Title: p.Title}
	}
	return app.OpenedPullRequest{Provider: provider, Number: p.Number, URL: p.URL, Title: p.Title}
}

func (g GitHosts) FindPullRequest(ctx context.Context, r domain.Repository, token, head, base string) (app.OpenedPullRequest, bool, error) {
	q := url.Values{}
	switch r.Provider {
	case domain.GitLab:
		q.Set("source_branch", head)
		q.Set("target_branch", base)
		q.Set("state", "opened")
		pr, found, _, err := g.findIn(ctx, r, token, r.API+"/merge_requests?"+q.Encode(), "", "")
		return pr, found, err
	case domain.GitHub:
		q.Set("head", r.Owner+":"+head)
		q.Set("base", base)
		q.Set("state", "open")
		pr, found, _, err := g.findIn(ctx, r, token, r.API+"/pulls?"+q.Encode(), "", "")
		return pr, found, err
	}
	// Forgejo and Gitea cannot filter by branch: page through the open
	// ones.
	for page := 1; page <= 20; page++ {
		q.Set("state", "open")
		q.Set("limit", "50")
		q.Set("page", strconv.Itoa(page))
		pr, found, last, err := g.findIn(ctx, r, token, r.API+"/pulls?"+q.Encode(), head, base)
		if err != nil || found || last {
			return pr, found, err
		}
	}
	return app.OpenedPullRequest{}, false, nil
}

// findIn answers the first Pull request of a list, or with head and base
// the first one between them; last is true when the page was empty.
func (g GitHosts) findIn(ctx context.Context, r domain.Repository, token, u, head, base string) (pr app.OpenedPullRequest, found, last bool, err error) {
	status, out, err := g.do(ctx, r.Provider, http.MethodGet, u, token, nil)
	if err != nil {
		return pr, false, false, err
	}
	if status == http.StatusNotFound {
		// Forgejo and Gitea answer 404 while a repository whose first
		// push just landed still counts as empty: it has none open.
		return pr, false, true, nil
	}
	if status/100 != 2 {
		return pr, false, false, fmt.Errorf("listing pull requests: %w", answered(r.Provider, status, out))
	}
	var list []pullRequest
	if err := json.Unmarshal(out, &list); err != nil {
		return pr, false, false, fmt.Errorf("%s answered an unreadable pull request list", r.Provider)
	}
	for _, p := range list {
		if head == "" || (p.Head.Ref == head && p.Base.Ref == base) {
			return p.opened(r.Provider), true, false, nil
		}
	}
	return pr, false, len(list) == 0, nil
}

func (g GitHosts) BranchExists(ctx context.Context, r domain.Repository, token, branch string) (bool, error) {
	u := r.API + "/branches/" + url.PathEscape(branch)
	if r.Provider == domain.GitLab {
		u = r.API + "/repository/branches/" + url.PathEscape(branch)
	}
	status, out, err := g.do(ctx, r.Provider, http.MethodGet, u, token, nil)
	switch {
	case err != nil:
		return false, err
	case status == http.StatusNotFound:
		return false, nil
	case status/100 != 2:
		return false, fmt.Errorf("reading branch %s: %w", branch, answered(r.Provider, status, out))
	}
	return true, nil
}

func (g GitHosts) CreatePullRequest(ctx context.Context, r domain.Repository, token, head, base, title, body string) (app.OpenedPullRequest, error) {
	u, payload := r.API+"/pulls", map[string]string{"head": head, "base": base, "title": title, "body": body}
	if r.Provider == domain.GitLab {
		u, payload = r.API+"/merge_requests", map[string]string{"source_branch": head, "target_branch": base, "title": title, "description": body}
	}
	status, out, err := g.do(ctx, r.Provider, http.MethodPost, u, token, payload)
	if err != nil {
		return app.OpenedPullRequest{}, err
	}
	if status/100 != 2 {
		return app.OpenedPullRequest{}, fmt.Errorf("opening a pull request: %w", answered(r.Provider, status, out))
	}
	var p pullRequest
	if err := json.Unmarshal(out, &p); err != nil {
		return app.OpenedPullRequest{}, fmt.Errorf("%s answered an unreadable pull request", r.Provider)
	}
	opened := p.opened(r.Provider)
	if opened.Number == 0 {
		return app.OpenedPullRequest{}, fmt.Errorf("%s answered a pull request without a number", r.Provider)
	}
	return opened, nil
}
