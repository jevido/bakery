package infra

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// fakePullRequests is a git host with one pushed branch and the Pull
// requests in open, answering in its Provider's shape.
func fakePullRequests(t *testing.T, p domain.Provider, open []map[string]any) (*httptest.Server, *[]string) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen = append(seen, r.Method+" "+r.URL.RequestURI()+" "+string(raw))
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/branches/"):
			if strings.HasSuffix(r.URL.Path, "bakery%2Fdef-12") || strings.HasSuffix(r.URL.Path, "bakery/def-12") {
				io.WriteString(w, `{}`)
				return
			}
			w.WriteHeader(404)
		case r.Method == "GET":
			if r.URL.Query().Get("page") > "1" {
				io.WriteString(w, `[]`)
				return
			}
			json.NewEncoder(w).Encode(open)
		case r.Method == "POST":
			w.WriteHeader(201)
			if p == domain.GitLab {
				io.WriteString(w, `{"iid":5,"web_url":"https://x/mr/5","title":"DEF-12 Fix"}`)
				return
			}
			io.WriteString(w, `{"number":5,"html_url":"https://x/pr/5","title":"DEF-12 Fix"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestGitHostsOpenPullRequest(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		p              domain.Provider
		api            string
		find, create   string
		branch, opened string
	}{
		{domain.Forgejo, "/api/v1/repos/e2e/shop", "GET /api/v1/repos/e2e/shop/pulls?limit=50&page=1&state=open ",
			`POST /api/v1/repos/e2e/shop/pulls {"base":"main","body":"b","head":"bakery/def-12","title":"t"}`,
			"GET /api/v1/repos/e2e/shop/branches/bakery%2Fdef-12 ", "https://x/pr/5"},
		{domain.GitHub, "/repos/acme/shop", "GET /repos/acme/shop/pulls?base=main&head=acme%3Abakery%2Fdef-12&state=open ",
			`POST /repos/acme/shop/pulls {"base":"main","body":"b","head":"bakery/def-12","title":"t"}`,
			"GET /repos/acme/shop/branches/bakery%2Fdef-12 ", "https://x/pr/5"},
		{domain.GitLab, "/api/v4/projects/acme%2Fshop", "GET /api/v4/projects/acme%2Fshop/merge_requests?source_branch=bakery%2Fdef-12&state=opened&target_branch=main ",
			`POST /api/v4/projects/acme%2Fshop/merge_requests {"description":"b","source_branch":"bakery/def-12","target_branch":"main","title":"t"}`,
			"GET /api/v4/projects/acme%2Fshop/repository/branches/bakery%2Fdef-12 ", "https://x/mr/5"},
	} {
		srv, seen := fakePullRequests(t, tc.p, nil)
		r := domain.Repository{Provider: tc.p, API: srv.URL + tc.api, Owner: "acme"}
		h := GitHosts{}
		if _, found, err := h.FindPullRequest(ctx, r, "tok", "bakery/def-12", "main"); err != nil || found {
			t.Fatalf("%s find: %v %v", tc.p, found, err)
		}
		if ok, err := h.BranchExists(ctx, r, "tok", "bakery/def-12"); err != nil || !ok {
			t.Fatalf("%s branch: %v %v", tc.p, ok, err)
		}
		if ok, err := h.BranchExists(ctx, r, "tok", "bakery/def-13"); err != nil || ok {
			t.Fatalf("%s missing branch: %v %v", tc.p, ok, err)
		}
		pr, err := h.CreatePullRequest(ctx, r, "tok", "bakery/def-12", "main", "t", "b")
		if err != nil || pr.Number != 5 || pr.URL != tc.opened || pr.Provider != tc.p {
			t.Fatalf("%s create: %+v %v", tc.p, pr, err)
		}
		got := *seen
		if got[0] != tc.find || got[1] != tc.branch || got[3] != tc.create {
			t.Errorf("%s calls:\n%s", tc.p, strings.Join(got, "\n"))
		}
	}
}

func TestGitHostsFindsTheOpenPullRequest(t *testing.T) {
	ctx := context.Background()
	open := []map[string]any{
		{"number": 3, "html_url": "https://x/pr/3", "title": "other", "head": map[string]any{"ref": "feature"}, "base": map[string]any{"ref": "main"}},
		{"number": 4, "html_url": "https://x/pr/4", "title": "into dev", "head": map[string]any{"ref": "bakery/def-12"}, "base": map[string]any{"ref": "dev"}},
		{"number": 5, "html_url": "https://x/pr/5", "title": "DEF-12", "head": map[string]any{"ref": "bakery/def-12"}, "base": map[string]any{"ref": "main"}},
	}
	srv, _ := fakePullRequests(t, domain.Gitea, open)
	pr, found, err := (GitHosts{}).FindPullRequest(ctx, domain.Repository{Provider: domain.Gitea, API: srv.URL}, "tok", "bakery/def-12", "main")
	if err != nil || !found || pr.Number != 5 || pr.URL != "https://x/pr/5" {
		t.Fatalf("gitea: %+v %v %v", pr, found, err)
	}
	srv, _ = fakePullRequests(t, domain.GitHub, open[2:])
	pr, found, err = (GitHosts{}).FindPullRequest(ctx, domain.Repository{Provider: domain.GitHub, API: srv.URL, Owner: "acme"}, "tok", "bakery/def-12", "main")
	if err != nil || !found || pr.Number != 5 {
		t.Fatalf("github: %+v %v %v", pr, found, err)
	}
}
