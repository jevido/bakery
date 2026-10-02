package http

import (
	"errors"
	"io"
	"mime"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

// maxWebhookBody bounds a push payload; GitHub caps its own at 25 MB but a
// push event for a branch is a few KB.
const maxWebhookBody = 1 << 20

type WebhookController struct {
	webhooks   *app.Webhooks
	isNotFound func(error) bool
}

func NewWebhookController(webhooks *app.Webhooks, isNotFound func(error) bool) *WebhookController {
	return &WebhookController{webhooks: webhooks, isNotFound: isNotFound}
}

type webhookJSON struct {
	// Path is appended to the dashboard's origin to get the URL.
	Path       string `json:"path"`
	Secret     string `json:"secret"`
	AutoDeploy bool   `json:"auto_deploy"`
	Previews   bool   `json:"previews"`
	// HasGitHostToken says whether a Git host token is saved; the token
	// itself is never returned.
	HasGitHostToken bool `json:"has_git_host_token"`
}

func (c *WebhookController) answer(ctx contractshttp.Context, w domain.Webhook) contractshttp.Response {
	return ctx.Response().Success().Json(contractshttp.Json{"webhook": webhookJSON{
		Path: WebhookPath(w.ApplicationID), Secret: w.Secret, AutoDeploy: w.AutoDeploy, Previews: w.Previews, HasGitHostToken: w.GitHostToken != "",
	}})
}

// WebhookPath is where a git host posts pushes for the Application.
func WebhookPath(applicationID uint64) string {
	return "/api/webhooks/applications/" + strconv.FormatUint(applicationID, 10)
}

func (c *WebhookController) fail(ctx contractshttp.Context, err error) contractshttp.Response {
	if errors.Is(err, app.ErrNotFound) || c.isNotFound(err) {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	return respond.ServerError(ctx, err)
}

func (c *WebhookController) Show(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	w, err := c.webhooks.Webhook(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return c.answer(ctx, w)
}

func (c *WebhookController) RotateSecret(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	w, err := c.webhooks.RotateSecret(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return c.answer(ctx, w)
}

func (c *WebhookController) Update(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	var req struct {
		AutoDeploy   *bool   `json:"auto_deploy"`
		Previews     *bool   `json:"previews"`
		GitHostToken *string `json:"git_host_token"`
	}
	if err := ctx.Request().Bind(&req); err != nil || (req.AutoDeploy == nil && req.Previews == nil && req.GitHostToken == nil) {
		return respond.Invalid(ctx, "auto_deploy", "auto_deploy, previews (true or false) or git_host_token is required")
	}
	if req.GitHostToken != nil && len(*req.GitHostToken) > 1000 {
		return respond.Invalid(ctx, "git_host_token", "git_host_token is at most 1000 characters")
	}
	var (
		w   domain.Webhook
		err error
	)
	if req.AutoDeploy != nil {
		if w, err = c.webhooks.SetAutoDeploy(ctx.Context(), id, *req.AutoDeploy); err != nil {
			return c.fail(ctx, err)
		}
	}
	if req.Previews != nil || req.GitHostToken != nil {
		if w, err = c.webhooks.SetPreviews(ctx.Context(), id, req.Previews, req.GitHostToken); err != nil {
			return c.fail(ctx, err)
		}
	}
	return c.answer(ctx, w)
}

// Receive is the git host's call. It carries no Session; the signature is
// the authentication.
func (c *WebhookController) Receive(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	req := ctx.Request().Origin()
	// Goravel parses form bodies before any handler runs and does not keep
	// the raw bytes, so a form-encoded payload can no longer be checked
	// against its signature. JSON bodies are put back after parsing.
	if mt, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type")); mt == "application/x-www-form-urlencoded" || mt == "multipart/form-data" {
		return respond.Error(ctx, contractshttp.StatusUnsupportedMediaType, "set the webhook's content type to application/json")
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxWebhookBody+1))
	if err != nil {
		return respond.Error(ctx, contractshttp.StatusBadRequest, "cannot read the body")
	}
	if len(body) > maxWebhookBody {
		return respond.Error(ctx, contractshttp.StatusRequestEntityTooLarge, "payload too large")
	}
	out, err := c.webhooks.ReceivePush(ctx.Context(), id, req.Header.Get, body)
	switch {
	case errors.Is(err, app.ErrBadSignature):
		return respond.Error(ctx, contractshttp.StatusUnauthorized, err.Error())
	case err != nil:
		return c.fail(ctx, err)
	case out.Deployment != nil:
		return ctx.Response().Json(contractshttp.StatusAccepted, contractshttp.Json{"deployment": ToJSON(*out.Deployment)})
	case out.Closed != 0:
		return ctx.Response().Success().Json(contractshttp.Json{"closed_preview": out.Closed})
	}
	return ctx.Response().Success().Json(contractshttp.Json{"ignored": out.Ignored})
}
