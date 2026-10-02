package domain

import "testing"

func TestParsePullRequest(t *testing.T) {
	cases := []struct {
		name     string
		provider Provider
		body     string
		want     PullRequest
	}{
		{"github opened", GitHub, `{"action":"opened","number":7,"pull_request":{"html_url":"https://github.com/o/r/pull/7","title":"Feature","head":{"ref":"feature","repo":{"full_name":"o/r"}},"base":{"ref":"main","repo":{"full_name":"o/r"}}},"repository":{"full_name":"o/r","url":"https://api.github.com/repos/o/r"}}`,
			PullRequest{Number: 7, Action: PullRequestOpened, Branch: "feature", Base: "main", Title: "Feature", URL: "https://github.com/o/r/pull/7", SameRepo: true, API: "https://api.github.com/repos/o/r"}},
		{"github synchronize from a fork", GitHub, `{"action":"synchronize","number":8,"pull_request":{"head":{"ref":"main","repo":{"full_name":"x/r"}},"base":{"ref":"main","repo":{"full_name":"o/r"}}},"repository":{"full_name":"o/r","url":"https://api.github.com/repos/o/r"}}`,
			PullRequest{Number: 8, Action: PullRequestPushed, Branch: "main", Base: "main", API: "https://api.github.com/repos/o/r"}},
		{"github labeled", GitHub, `{"action":"labeled","number":7,"pull_request":{"head":{"ref":"f","repo":{"full_name":"o/r"}},"base":{"ref":"main"}},"repository":{"full_name":"o/r","url":"https://api.github.com/repos/o/r"}}`,
			PullRequest{Number: 7, Branch: "f", Base: "main", SameRepo: true, API: "https://api.github.com/repos/o/r"}},
		{"forgejo synchronized", Forgejo, `{"action":"synchronized","number":3,"pull_request":{"html_url":"http://127.0.0.1:4950/u/r/pulls/3","title":"T","head":{"ref":"feature","repo_id":5},"base":{"ref":"main","repo_id":5}},"repository":{"full_name":"u/r","html_url":"http://127.0.0.1:4950/u/r"}}`,
			PullRequest{Number: 3, Action: PullRequestPushed, Branch: "feature", Base: "main", Title: "T", URL: "http://127.0.0.1:4950/u/r/pulls/3", SameRepo: true, API: "http://127.0.0.1:4950/api/v1/repos/u/r"}},
		{"gitea closed under a sub path, fork", Gitea, `{"action":"closed","number":4,"pull_request":{"head":{"ref":"f","repo_id":6},"base":{"ref":"main","repo_id":5}},"repository":{"full_name":"u/r","html_url":"https://example.com/git/u/r"}}`,
			PullRequest{Number: 4, Action: PullRequestClosed, Branch: "f", Base: "main", API: "https://example.com/git/api/v1/repos/u/r"}},
		{"gitlab update with new commits", GitLab, `{"object_kind":"merge_request","object_attributes":{"iid":2,"action":"update","oldrev":"abc","source_branch":"f","target_branch":"main","source_project_id":9,"target_project_id":9,"url":"https://gitlab.com/o/r/-/merge_requests/2","title":"T"},"project":{"id":9,"web_url":"https://gitlab.com/o/r"}}`,
			PullRequest{Number: 2, Action: PullRequestPushed, Branch: "f", Base: "main", Title: "T", URL: "https://gitlab.com/o/r/-/merge_requests/2", SameRepo: true, API: "https://gitlab.com/api/v4/projects/9"}},
		{"gitlab title edit", GitLab, `{"object_kind":"merge_request","object_attributes":{"iid":2,"action":"update","source_branch":"f","target_branch":"main","source_project_id":9,"target_project_id":9},"project":{"id":9,"web_url":"https://gitlab.com/o/r"}}`,
			PullRequest{Number: 2, Branch: "f", Base: "main", SameRepo: true, API: "https://gitlab.com/api/v4/projects/9"}},
		{"gitlab merge", GitLab, `{"object_kind":"merge_request","object_attributes":{"iid":2,"action":"merge","source_branch":"f","target_branch":"main","source_project_id":9,"target_project_id":9},"project":{"id":9,"web_url":"https://gitlab.com/o/r"}}`,
			PullRequest{Number: 2, Action: PullRequestClosed, Branch: "f", Base: "main", SameRepo: true, API: "https://gitlab.com/api/v4/projects/9"}},
	}
	for _, c := range cases {
		got, err := ParsePullRequest(c.provider, []byte(c.body))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
	if _, err := ParsePullRequest(GitHub, []byte(`{"ref":"refs/heads/main"}`)); err != ErrNotAPullRequest {
		t.Errorf("a push payload: %v", err)
	}
	if !IsPullRequest(Forgejo, "pull_request") || !IsPullRequest(GitLab, "Merge Request Hook") || IsPullRequest(GitHub, "push") {
		t.Error("IsPullRequest")
	}
}
