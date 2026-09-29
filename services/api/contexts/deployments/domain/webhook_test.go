package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

func sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func TestWebhookVerify(t *testing.T) {
	w := Webhook{Secret: "s3cret"}
	body := []byte(`{"ref":"refs/heads/main"}`)
	good, bad := sign("s3cret", body), sign("other", body)

	cases := []struct {
		name    string
		headers map[string]string
		want    Provider
		ok      bool
	}{
		{"github", map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=" + good}, GitHub, true},
		{"github wrong secret", map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=" + bad}, GitHub, false},
		{"github unsigned", map[string]string{"X-GitHub-Event": "push"}, GitHub, false},
		{"github gitea header ignored", map[string]string{"X-GitHub-Event": "push", "X-Gitea-Signature": good}, GitHub, false},
		{"gitea", map[string]string{"X-Gitea-Event": "push", "X-Gitea-Signature": good}, Gitea, true},
		{"forgejo", map[string]string{"X-Forgejo-Event": "push", "X-Gitea-Event": "push", "X-Forgejo-Signature": good}, Forgejo, true},
		{"forgejo hub header", map[string]string{"X-Forgejo-Event": "push", "X-Hub-Signature-256": "sha256=" + good}, Forgejo, true},
		{"gitlab", map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": "s3cret"}, GitLab, true},
		{"gitlab wrong", map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": "nope"}, GitLab, false},
	}
	for _, c := range cases {
		h := http.Header{}
		for k, v := range c.headers {
			h.Set(k, v)
		}
		p, event := DetectProvider(h.Get)
		if p != c.want || !IsPush(p, event) {
			t.Errorf("%s: provider %q event %q", c.name, p, event)
		}
		if got := w.Verify(p, h.Get, body); got != c.ok {
			t.Errorf("%s: verify = %v, want %v", c.name, got, c.ok)
		}
	}
	if (Webhook{}).Verify(GitLab, http.Header{"X-Gitlab-Token": {""}}.Get, body) {
		t.Error("an empty secret must never verify")
	}
	if p, _ := DetectProvider(http.Header{}.Get); p != "" {
		t.Errorf("no headers: %q", p)
	}
}

func TestPushedBranch(t *testing.T) {
	if b, err := PushedBranch([]byte(`{"ref":"refs/heads/feature/x","after":"abc"}`)); err != nil || b != "feature/x" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := PushedBranch([]byte(`{"ref":"refs/tags/v1"}`)); err != ErrNotABranchPush {
		t.Fatalf("tag: %v", err)
	}
	if _, err := PushedBranch([]byte(`not json`)); err == nil {
		t.Fatal("bad json accepted")
	}
}
