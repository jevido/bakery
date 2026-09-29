package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
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
	s := newSetup(t, fakeSource{})
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
