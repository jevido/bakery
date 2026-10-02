package infra

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type call struct{ method, path, auth, gitlab, accept, body string }

func gitHost(t *testing.T, status int, answer string) (*httptest.Server, *[]call) {
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b struct{ Body string }
		json.Unmarshal(raw, &b)
		calls = append(calls, call{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("PRIVATE-TOKEN"), r.Header.Get("Accept"), b.Body})
		w.WriteHeader(status)
		io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestGitHostsComment(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		provider                       domain.Provider
		api                            string
		postPath, editPath, editMethod string
		check                          func(c call) bool
	}{
		{domain.Forgejo, "/api/v1/repos/u/r", "/api/v1/repos/u/r/issues/7/comments", "/api/v1/repos/u/r/issues/comments/42", "PATCH",
			func(c call) bool { return c.auth == "token tok" }},
		{domain.GitHub, "/repos/o/r", "/repos/o/r/issues/7/comments", "/repos/o/r/issues/comments/42", "PATCH",
			func(c call) bool { return c.auth == "Bearer tok" && c.accept == "application/vnd.github+json" }},
		{domain.GitLab, "/api/v4/projects/9", "/api/v4/projects/9/merge_requests/7/notes", "/api/v4/projects/9/merge_requests/7/notes/42", "PUT",
			func(c call) bool { return c.gitlab == "tok" && c.auth == "" }},
	}
	for _, tc := range cases {
		srv, calls := gitHost(t, 201, `{"id":42}`)
		target := app.CommentTarget{Provider: tc.provider, API: srv.URL + tc.api, Number: 7}
		id, err := (GitHosts{}).Post(ctx, target, "tok", "hello")
		if err != nil || id != "42" {
			t.Fatalf("%s post: %q %v", tc.provider, id, err)
		}
		if err := (GitHosts{}).Edit(ctx, target, "tok", "42", "again"); err != nil {
			t.Fatalf("%s edit: %v", tc.provider, err)
		}
		post, edit := (*calls)[0], (*calls)[1]
		if post.method != "POST" || post.path != tc.postPath || post.body != "hello" || !tc.check(post) {
			t.Errorf("%s post call %+v", tc.provider, post)
		}
		if edit.method != tc.editMethod || edit.path != tc.editPath || edit.body != "again" || !tc.check(edit) {
			t.Errorf("%s edit call %+v", tc.provider, edit)
		}
	}
}

func TestGitHostsErrors(t *testing.T) {
	ctx := context.Background()
	srv, _ := gitHost(t, 404, `{"message":"Not Found"}`)
	target := app.CommentTarget{Provider: domain.Gitea, API: srv.URL, Number: 1}
	if err := (GitHosts{}).Edit(ctx, target, "t", "5", "x"); !errors.Is(err, app.ErrCommentGone) {
		t.Errorf("edit of a deleted comment: %v", err)
	}
	if _, err := (GitHosts{}).Post(ctx, target, "t", "x"); err == nil || errors.Is(err, app.ErrCommentGone) {
		t.Errorf("post to a missing pull request: %v", err)
	}
	srv, _ = gitHost(t, 401, `{"message":"token is required"}`)
	target.API = srv.URL
	if _, err := (GitHosts{}).Post(ctx, target, "t", "x"); err == nil || err.Error() != `gitea answered 401: {"message":"token is required"}` {
		t.Errorf("unauthorised: %v", err)
	}
}
