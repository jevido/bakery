package domain

import (
	"errors"
	"testing"
)

func TestRepositoryOf(t *testing.T) {
	cases := []struct {
		remembered Provider
		url        string
		want       Repository
	}{
		{"", "https://github.com/acme/shop.git", Repository{GitHub, "https://api.github.com/repos/acme/shop", "acme"}},
		{"", "git@github.com:acme/shop.git", Repository{GitHub, "https://api.github.com/repos/acme/shop", "acme"}},
		{GitHub, "https://ghe.example.com/acme/shop", Repository{GitHub, "https://ghe.example.com/api/v3/repos/acme/shop", "acme"}},
		{Forgejo, "http://127.0.0.1:4950/e2e/shop.git", Repository{Forgejo, "http://127.0.0.1:4950/api/v1/repos/e2e/shop", "e2e"}},
		{Gitea, "ssh://git@git.example.com:2222/acme/shop.git", Repository{Gitea, "https://git.example.com/api/v1/repos/acme/shop", "acme"}},
		{Gitea, "https://example.com/git/acme/shop", Repository{Gitea, "https://example.com/git/api/v1/repos/acme/shop", "acme"}},
		{"", "https://gitlab.com/acme/web/shop.git", Repository{GitLab, "https://gitlab.com/api/v4/projects/acme%2Fweb%2Fshop", "acme/web"}},
		{GitLab, "git@code.example.com:acme/shop.git", Repository{GitLab, "https://code.example.com/api/v4/projects/acme%2Fshop", "acme"}},
	}
	for _, c := range cases {
		got, err := RepositoryOf(c.remembered, c.url)
		if err != nil || got != c.want {
			t.Errorf("RepositoryOf(%q, %q) = %+v, %v; want %+v", c.remembered, c.url, got, err, c.want)
		}
	}
	if _, err := RepositoryOf("", "https://git.example.com/acme/shop.git"); !errors.Is(err, ErrUnknownGitHost) {
		t.Errorf("unknown host: %v", err)
	}
	for _, bad := range []string{"", "https://github.com/shop", "file:///srv/shop.git", "github.com/acme"} {
		if _, err := RepositoryOf(GitHub, bad); err == nil || errors.Is(err, ErrUnknownGitHost) {
			t.Errorf("RepositoryOf(%q) = %v, want a bad URL", bad, err)
		}
	}
}

func TestRepositoryAPI(t *testing.T) {
	cases := []struct {
		p    Provider
		body string
		want string
	}{
		{GitHub, `{"ref":"refs/heads/main","repository":{"url":"https://github.com/acme/shop","pulls_url":"https://api.github.com/repos/acme/shop/pulls{/number}"}}`, "https://api.github.com/repos/acme/shop"},
		{GitHub, `{"repository":{"pulls_url":"https://ghe.example.com/api/v3/repos/acme/shop/pulls{/number}"}}`, "https://ghe.example.com/api/v3/repos/acme/shop"},
		{GitHub, `{"zen":"ping"}`, ""},
		{Forgejo, `{"ref":"refs/heads/main","repository":{"full_name":"e2e/shop","html_url":"http://127.0.0.1:4950/e2e/shop"}}`, "http://127.0.0.1:4950/api/v1/repos/e2e/shop"},
		{Gitea, `{"repository":{"full_name":"acme/shop","html_url":"https://example.com/git/acme/shop/"}}`, "https://example.com/git/api/v1/repos/acme/shop"},
		{GitLab, `{"object_kind":"push","project":{"id":42,"web_url":"https://code.example.com/acme/shop"}}`, "https://code.example.com/api/v4/projects/42"},
		{GitLab, `not json`, ""},
	}
	for _, c := range cases {
		if got := RepositoryAPI(c.p, []byte(c.body)); got != c.want {
			t.Errorf("RepositoryAPI(%s, %s) = %q; want %q", c.p, c.body, got, c.want)
		}
	}
}

func TestWebhookRepository(t *testing.T) {
	hook := Webhook{Provider: Forgejo, RepositoryAPI: "http://127.0.0.1:4950/api/v1/repos/e2e/shop"}
	got, err := hook.Repository("ssh://git@127.0.0.1:4952/e2e/shop.git")
	if err != nil || got != (Repository{Forgejo, "http://127.0.0.1:4950/api/v1/repos/e2e/shop", "e2e"}) {
		t.Fatalf("remembered: %+v %v", got, err)
	}
	hook.RepositoryAPI = ""
	if got, _ := hook.Repository("ssh://git@127.0.0.1:4952/e2e/shop.git"); got.API != "https://127.0.0.1/api/v1/repos/e2e/shop" {
		t.Fatalf("derived: %+v", got)
	}
}
