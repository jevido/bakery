package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// Webhook lets a git host deploy one Application on push.
type Webhook struct {
	ApplicationID uint64
	Secret        string
	AutoDeploy    bool
}

// Provider is the git host that called a Webhook, recognised by its event
// header.
type Provider string

const (
	GitHub  Provider = "github"
	Gitea   Provider = "gitea"
	Forgejo Provider = "forgejo"
	GitLab  Provider = "gitlab"
)

// Header reads one request header, case-insensitively.
type Header func(name string) string

// DetectProvider returns the Provider and its event name, or "" when no
// known event header is present. Forgejo also sends the Gitea headers, so
// it is checked first.
func DetectProvider(h Header) (Provider, string) {
	switch {
	case h("X-Forgejo-Event") != "":
		return Forgejo, h("X-Forgejo-Event")
	case h("X-Gitea-Event") != "":
		return Gitea, h("X-Gitea-Event")
	case h("X-GitHub-Event") != "":
		return GitHub, h("X-GitHub-Event")
	case h("X-Gitlab-Event") != "":
		return GitLab, h("X-Gitlab-Event")
	}
	return "", ""
}

// IsPush reports whether event is the Provider's push event.
func IsPush(p Provider, event string) bool {
	if p == GitLab {
		return event == "Push Hook"
	}
	return event == "push"
}

// Verify reports whether the call was signed with the Webhook's secret:
// an HMAC-SHA256 of the raw body for GitHub, Gitea and Forgejo, the token
// itself for GitLab. Comparisons take constant time.
func (w Webhook) Verify(p Provider, h Header, body []byte) bool {
	if w.Secret == "" {
		return false
	}
	if p == GitLab {
		token := h("X-Gitlab-Token")
		return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(w.Secret)) == 1
	}
	mac := hmac.New(sha256.New, []byte(w.Secret))
	mac.Write(body)
	want := mac.Sum(nil)
	candidates := []string{strings.TrimPrefix(h("X-Hub-Signature-256"), "sha256=")}
	if p == Gitea || p == Forgejo {
		candidates = append(candidates, h("X-Gitea-Signature"), h("X-Forgejo-Signature"))
	}
	for _, c := range candidates {
		got, err := hex.DecodeString(strings.TrimSpace(c))
		if err == nil && len(got) > 0 && hmac.Equal(got, want) {
			return true
		}
	}
	return false
}

var ErrNotABranchPush = errors.New("not a push to a branch")

// PushedBranch reads the branch a push event went to from its "ref"
// (refs/heads/<branch>), the same field for every Provider. Tag pushes are
// ErrNotABranchPush.
func PushedBranch(body []byte) (string, error) {
	var push struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(body, &push); err != nil {
		return "", err
	}
	branch, ok := strings.CutPrefix(push.Ref, "refs/heads/")
	if !ok || branch == "" {
		return "", ErrNotABranchPush
	}
	return branch, nil
}
