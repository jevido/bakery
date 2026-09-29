package http

import (
	"errors"
	"io"
	"mime"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
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
}

func (c *WebhookController) answer(ctx contractshttp.Context, applicationID uint64, secret string, autoDeploy bool) contractshttp.Response {
	return ctx.Response().Success().Json(contractshttp.Json{"webhook": webhookJSON{
		Path: WebhookPath(applicationID), Secret: secret, AutoDeploy: autoDeploy,
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
	return c.answer(ctx, id, w.Secret, w.AutoDeploy)
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
	return c.answer(ctx, id, w.Secret, w.AutoDeploy)
}

func (c *WebhookController) Update(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	var req struct {
		AutoDeploy *bool `json:"auto_deploy"`
	}
	if err := ctx.Request().Bind(&req); err != nil || req.AutoDeploy == nil {
		return respond.Invalid(ctx, "auto_deploy", "auto_deploy (true or false) is required")
	}
	w, err := c.webhooks.SetAutoDeploy(ctx.Context(), id, *req.AutoDeploy)
	if err != nil {
		return c.fail(ctx, err)
	}
	return c.answer(ctx, id, w.Secret, w.AutoDeploy)
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
	}
	return ctx.Response().Success().Json(contractshttp.Json{"ignored": out.Ignored})
}
