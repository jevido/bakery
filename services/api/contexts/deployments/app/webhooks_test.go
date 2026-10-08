package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type memWebhooks map[uint64]domain.Webhook

func (m memWebhooks) ByApplication(_ context.Context, id uint64) (domain.Webhook, bool, error) {
	w, ok := m[id]
	return w, ok, nil
}
func (m memWebhooks) Save(_ context.Context, w domain.Webhook) error {
	m[w.ApplicationID] = w
	return nil
}
func (m memWebhooks) RememberRepository(_ context.Context, id uint64, p domain.Provider, api string) error {
	w := m[id]
	w.Provider, w.RepositoryAPI = p, api
	m[id] = w
	return nil
}
func (m memWebhooks) DeleteForApplication(_ context.Context, id uint64) error {
	delete(m, id)
	return nil
}

func githubPush(secret, ref string) (domain.Header, []byte) {
	body := []byte(`{"ref":"` + ref + `"}`)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	h := http.Header{}
	h.Set("X-GitHub-Event", "push")
	h.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(m.Sum(nil)))
	return h.Get, body
}

func TestReceivePush(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeCloner{})
	hooks := NewWebhooks(s.service, memWebhooks{})

	h, body := githubPush("x", "refs/heads/main")
	if _, err := hooks.ReceivePush(ctx, 1, h, body); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before the Owner opened the webhook: %v", err)
	}
	hook, err := hooks.Webhook(ctx, 1)
	if err != nil || len(hook.Secret) != 64 || !hook.AutoDeploy {
		t.Fatalf("webhook: %+v %v", hook, err)
	}
	if again, _ := hooks.Webhook(ctx, 1); again.Secret != hook.Secret {
		t.Fatal("reading the webhook must not change its secret")
	}

	h, body = githubPush("wrong", "refs/heads/main")
	if _, err := hooks.ReceivePush(ctx, 1, h, body); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("wrong secret: %v", err)
	}
	h, body = githubPush(hook.Secret, "refs/heads/other")
	if out, err := hooks.ReceivePush(ctx, 1, h, body); err != nil || out.Deployment != nil || out.Ignored == "" {
		t.Fatalf("other branch: %+v %v", out, err)
	}

	h, body = githubPush(hook.Secret, "refs/heads/main")
	out, err := hooks.ReceivePush(ctx, 1, h, body)
	if err != nil || out.Deployment == nil || out.Deployment.Trigger != domain.TriggerWebhook {
		t.Fatalf("push: %+v %v", out, err)
	}
	if out, err := hooks.ReceivePush(ctx, 1, h, body); err != nil || out.Ignored != "a deployment is already queued" {
		t.Fatalf("second push while queued: %+v %v", out, err)
	}

	if _, err := hooks.SetAutoDeploy(ctx, 1, false); err != nil {
		t.Fatal(err)
	}
	s.worker.RunOnce(ctx)
	if out, _ := hooks.ReceivePush(ctx, 1, h, body); out.Ignored != "auto-deploy is off" {
		t.Fatalf("auto-deploy off: %+v", out)
	}

	rotated, _ := hooks.RotateSecret(ctx, 1)
	if rotated.Secret == hook.Secret || rotated.AutoDeploy {
		t.Fatalf("rotate: %+v", rotated)
	}
	if _, err := hooks.ReceivePush(ctx, 1, h, body); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("old secret after rotating: %v", err)
	}
}

func TestReceivePushIgnoresImageApplications(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeCloner{})
	s.app = imageApp
	hooks := NewWebhooks(s.service, memWebhooks{})
	hook, _ := hooks.Webhook(ctx, 1)
	h, body := githubPush(hook.Secret, "refs/heads/main")
	if out, err := hooks.ReceivePush(ctx, 1, h, body); err != nil || out.Deployment != nil || out.Ignored != "dockerimage applications are not built from git" {
		t.Fatalf("push: %+v %v", out, err)
	}
}

func forgejoPullRequest(secret, action string, number int, head, base string, headRepo int) (domain.Header, []byte) {
	body := []byte(fmt.Sprintf(`{"action":%q,"number":%d,"pull_request":{"html_url":"http://git/u/r/pulls/%d","title":"T","head":{"ref":%q,"repo_id":%d},"base":{"ref":%q,"repo_id":5}},"repository":{"full_name":"u/r","html_url":"http://git/u/r"}}`,
		action, number, number, head, headRepo, base))
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	h := http.Header{}
	h.Set("X-Forgejo-Event", "pull_request")
	h.Set("X-Forgejo-Signature", hex.EncodeToString(m.Sum(nil)))
	return h.Get, body
}

func TestReceivePullRequest(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeCloner{})
	hooks := NewWebhooks(s.service, memWebhooks{})
	hook, _ := hooks.Webhook(ctx, 1)
	var told []string
	hooks.PullRequestChanged = func(_ context.Context, a Application, p domain.Provider, pr domain.PullRequest) {
		told = append(told, fmt.Sprintf("%d %s #%d %s", a.ID, p, pr.Number, pr.Action))
	}
	call := func(action string, number int, head, base string, headRepo int) PushOutcome {
		t.Helper()
		h, body := forgejoPullRequest(hook.Secret, action, number, head, base, headRepo)
		out, err := hooks.ReceivePush(ctx, 1, h, body)
		if err != nil {
			t.Fatalf("%s #%d: %v", action, number, err)
		}
		return out
	}

	if out := call("opened", 7, "feature", "main", 5); out.Ignored != "previews are off" {
		t.Fatalf("previews off: %+v", out)
	}
	// Told before the Previews filters, so work follows it all the same.
	if len(told) != 1 || told[0] != "1 forgejo #7 opened" {
		t.Fatalf("told %v", told)
	}
	on, token := true, "tok"
	if hook, _ = hooks.SetPreviews(ctx, 1, &on, &token); !hook.Previews || hook.GitHostToken != "tok" || !hook.AutoDeploy {
		t.Fatalf("set previews: %+v", hook)
	}
	if out := call("opened", 8, "feature", "main", 6); !strings.Contains(out.Ignored, "fork") {
		t.Fatalf("fork: %+v", out)
	}
	if out := call("opened", 9, "feature", "develop", 5); !strings.Contains(out.Ignored, "merges into develop") {
		t.Fatalf("other base: %+v", out)
	}
	if out := call("edited", 7, "feature", "main", 5); !strings.Contains(out.Ignored, "nothing to do") {
		t.Fatalf("edited: %+v", out)
	}
	if len(told) != 3 {
		t.Fatalf("an edit is not told: %v", told)
	}

	out := call("opened", 7, "feature", "main", 5)
	if out.Deployment == nil || out.Deployment.Preview != 7 || out.Deployment.Trigger != domain.TriggerWebhook {
		t.Fatalf("opened: %+v", out)
	}
	p, _, _ := s.previews.ByNumber(ctx, 1, 7)
	if p.State != domain.PreviewOpen || p.Branch != "feature" || p.API != "http://git/api/v1/repos/u/r" || p.Provider != domain.Forgejo {
		t.Fatalf("preview: %+v", p)
	}
	if out := call("synchronized", 7, "feature", "main", 5); out.Ignored != "a deployment of this preview is already queued" {
		t.Fatalf("pushed while queued: %+v", out)
	}
	s.worker.RunOnce(ctx)
	if out := call("synchronized", 7, "feature", "main", 5); out.Deployment == nil {
		t.Fatalf("pushed: %+v", out)
	}
	s.worker.RunOnce(ctx)

	if out := call("closed", 7, "feature", "main", 5); out.Closed != 7 {
		t.Fatalf("closed: %+v", out)
	}
	if p, _, _ := s.previews.ByNumber(ctx, 1, 7); p.State != domain.PreviewClosed {
		t.Fatalf("after close: %+v", p)
	}
	if out := call("closed", 7, "feature", "main", 5); !strings.Contains(out.Ignored, "no open preview") {
		t.Fatalf("closed twice: %+v", out)
	}
	if out := call("reopened", 7, "feature", "main", 5); out.Deployment == nil {
		t.Fatalf("reopened: %+v", out)
	}
	if p, _, _ := s.previews.ByNumber(ctx, 1, 7); p.State != domain.PreviewOpen || p.ClosedAt != nil {
		t.Fatalf("after reopen: %+v", p)
	}

	// A push still deploys the Application itself.
	h, body := githubPush(hook.Secret, "refs/heads/main")
	if out, err := hooks.ReceivePush(ctx, 1, h, body); err != nil || out.Deployment == nil || out.Deployment.Preview != 0 {
		t.Fatalf("push: %+v %v", out, err)
	}
}
