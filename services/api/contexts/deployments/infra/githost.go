package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// GitHosts writes Preview comments through the REST APIs of Forgejo and
// Gitea, GitHub and GitLab. The API base comes from the Pull request event.
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
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	switch t.Provider {
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
		return nil, err
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode == http.StatusNotFound && method != http.MethodPost {
		return nil, app.ErrCommentGone
	}
	if res.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return nil, fmt.Errorf("%s answered %d: %s", t.Provider, res.StatusCode, msg)
	}
	return out, nil
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
