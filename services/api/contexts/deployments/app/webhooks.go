package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

var ErrBadSignature = errors.New("the webhook signature does not match")

// WebhookStore keeps one Webhook per Application; the secret is encrypted
// at rest by the store.
type WebhookStore interface {
	ByApplication(ctx context.Context, applicationID uint64) (domain.Webhook, bool, error)
	// Save creates or replaces the Application's Webhook.
	Save(ctx context.Context, w domain.Webhook) error
	DeleteForApplication(ctx context.Context, applicationID uint64) error
}

// Webhooks are the use cases around an Application's Webhook.
type Webhooks struct {
	service *Service
	store   WebhookStore
}

func NewWebhooks(service *Service, store WebhookStore) *Webhooks {
	return &Webhooks{service: service, store: store}
}

func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Webhook returns the Application's Webhook, creating it (Auto-deploy on,
// fresh secret) the first time the Owner asks for it.
func (w *Webhooks) Webhook(ctx context.Context, applicationID uint64) (domain.Webhook, error) {
	if _, err := w.service.applications(ctx, applicationID); err != nil {
		return domain.Webhook{}, err
	}
	hook, found, err := w.store.ByApplication(ctx, applicationID)
	if err != nil || found {
		return hook, err
	}
	secret, err := newSecret()
	if err != nil {
		return domain.Webhook{}, err
	}
	hook = domain.Webhook{ApplicationID: applicationID, Secret: secret, AutoDeploy: true}
	return hook, w.store.Save(ctx, hook)
}

// RotateSecret gives the Webhook a new secret; calls signed with the old one
// are refused from now on.
func (w *Webhooks) RotateSecret(ctx context.Context, applicationID uint64) (domain.Webhook, error) {
	hook, err := w.Webhook(ctx, applicationID)
	if err != nil {
		return domain.Webhook{}, err
	}
	if hook.Secret, err = newSecret(); err != nil {
		return domain.Webhook{}, err
	}
	return hook, w.store.Save(ctx, hook)
}

func (w *Webhooks) SetAutoDeploy(ctx context.Context, applicationID uint64, on bool) (domain.Webhook, error) {
	hook, err := w.Webhook(ctx, applicationID)
	if err != nil {
		return domain.Webhook{}, err
	}
	hook.AutoDeploy = on
	return hook, w.store.Save(ctx, hook)
}

// SetPreviews switches Previews on or off and, when token is not nil,
// replaces the Git host token ("" removes it).
func (w *Webhooks) SetPreviews(ctx context.Context, applicationID uint64, on *bool, token *string) (domain.Webhook, error) {
	hook, err := w.Webhook(ctx, applicationID)
	if err != nil {
		return domain.Webhook{}, err
	}
	if on != nil {
		hook.Previews = *on
	}
	if token != nil {
		hook.GitHostToken = *token
	}
	return hook, w.store.Save(ctx, hook)
}

func (w *Webhooks) DeleteForApplication(ctx context.Context, applicationID uint64) error {
	return w.store.DeleteForApplication(ctx, applicationID)
}

// PushOutcome is what ReceivePush did: queued a Deployment, closed a
// Preview, or ignored the call for Ignored's reason.
type PushOutcome struct {
	Deployment *domain.Deployment
	// Closed is the number of the Preview a closed Pull request removed.
	Closed  int
	Ignored string
}

// ReceivePush handles a git host's call. An unknown Application and an
// Application without a Webhook are both ErrNotFound, so the endpoint does
// not tell which Applications exist. Only a verified push to the
// Application's branch with Auto-deploy on queues a Deployment.
func (w *Webhooks) ReceivePush(ctx context.Context, applicationID uint64, header domain.Header, body []byte) (PushOutcome, error) {
	hook, found, err := w.store.ByApplication(ctx, applicationID)
	if err != nil {
		return PushOutcome{}, err
	}
	if !found {
		return PushOutcome{}, ErrNotFound
	}
	app, err := w.service.applications(ctx, applicationID)
	if err != nil {
		return PushOutcome{}, err
	}
	provider, event := domain.DetectProvider(header)
	if provider == "" || !hook.Verify(provider, header, body) {
		return PushOutcome{}, ErrBadSignature
	}
	if domain.IsPullRequest(provider, event) {
		return w.receivePullRequest(ctx, hook, app, provider, body)
	}
	if !domain.IsPush(provider, event) {
		return PushOutcome{Ignored: fmt.Sprintf("%s event", event)}, nil
	}
	if app.BuildPack == BuildPackImage {
		return PushOutcome{Ignored: "image applications are not built from git"}, nil
	}
	if !hook.AutoDeploy {
		return PushOutcome{Ignored: "auto-deploy is off"}, nil
	}
	branch, err := domain.PushedBranch(body)
	if err != nil {
		return PushOutcome{Ignored: "not a push to a branch"}, nil
	}
	if branch != app.GitBranch {
		return PushOutcome{Ignored: fmt.Sprintf("push to %s, the application deploys %s", branch, app.GitBranch)}, nil
	}
	d, err := w.service.queue(ctx, domain.NewDeployment(applicationID, domain.TriggerWebhook))
	if errors.Is(err, domain.ErrAlreadyQueued) {
		return PushOutcome{Ignored: "a deployment is already queued"}, nil
	}
	if err != nil {
		return PushOutcome{}, err
	}
	return PushOutcome{Deployment: &d}, nil
}

// receivePullRequest opens, deploys or closes a Preview for a verified
// Pull request event.
func (w *Webhooks) receivePullRequest(ctx context.Context, hook domain.Webhook, app Application, provider domain.Provider, body []byte) (PushOutcome, error) {
	pr, err := domain.ParsePullRequest(provider, body)
	if err != nil {
		return PushOutcome{Ignored: "not a pull request"}, nil
	}
	switch {
	case !hook.Previews:
		return PushOutcome{Ignored: "previews are off"}, nil
	case app.BuildPack == BuildPackImage:
		return PushOutcome{Ignored: "image applications have no previews"}, nil
	case pr.Action == "":
		return PushOutcome{Ignored: fmt.Sprintf("nothing to do for pull request #%d", pr.Number)}, nil
	case !pr.SameRepo:
		return PushOutcome{Ignored: fmt.Sprintf("pull request #%d comes from a fork", pr.Number)}, nil
	case pr.Base != app.GitBranch:
		return PushOutcome{Ignored: fmt.Sprintf("pull request #%d merges into %s, the application deploys %s", pr.Number, pr.Base, app.GitBranch)}, nil
	}
	p, found, err := w.service.previews.ByNumber(ctx, app.ID, pr.Number)
	if err != nil {
		return PushOutcome{}, err
	}
	if pr.Action == domain.PullRequestClosed {
		if !found || p.State == domain.PreviewClosed {
			return PushOutcome{Ignored: fmt.Sprintf("pull request #%d has no open preview", pr.Number)}, nil
		}
		if err := w.service.ClosePreview(ctx, app.ID, pr.Number); err != nil {
			return PushOutcome{}, err
		}
		return PushOutcome{Closed: pr.Number}, nil
	}
	if !found {
		p = domain.Preview{ApplicationID: app.ID, Number: pr.Number, State: domain.PreviewOpen}
	}
	if p.State == domain.PreviewClosed {
		p.Reopen()
	}
	p.Branch, p.Title, p.URL, p.Provider, p.API = pr.Branch, pr.Title, pr.URL, provider, pr.API
	if _, err := w.service.previews.Save(ctx, p); err != nil {
		return PushOutcome{}, err
	}
	d, err := w.service.DeployPreview(ctx, app.ID, pr.Number, domain.TriggerWebhook)
	if errors.Is(err, domain.ErrAlreadyQueued) {
		return PushOutcome{Ignored: "a deployment of this preview is already queued"}, nil
	}
	if err != nil {
		return PushOutcome{}, err
	}
	return PushOutcome{Deployment: &d}, nil
}
