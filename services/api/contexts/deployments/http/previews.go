package http

import (
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type previewJSON struct {
	Number    int             `json:"number"`
	Title     string          `json:"title"`
	Branch    string          `json:"branch"`
	URL       string          `json:"url"`
	Provider  string          `json:"provider"`
	State     string          `json:"state"`
	Domain    string          `json:"domain"`
	PublicURL string          `json:"public_url"`
	Commented bool            `json:"commented"`
	Latest    *deploymentJSON `json:"latest_deployment"`
	CreatedAt time.Time       `json:"created_at"`
	ClosedAt  *time.Time      `json:"closed_at"`
}

func previewToJSON(p app.PreviewView) previewJSON {
	out := previewJSON{
		Number: p.Number, Title: p.Title, Branch: p.Branch, URL: p.URL, Provider: string(p.Provider), State: string(p.State),
		Domain: p.Domain, Commented: p.CommentID != "", CreatedAt: p.CreatedAt, ClosedAt: p.ClosedAt,
	}
	if p.Latest != nil {
		d := ToJSON(*p.Latest)
		out.Latest = &d
	}
	if p.Domain != "" && PublicURL != nil {
		var server uint64
		if p.Latest != nil {
			server = p.Latest.ServerID
		}
		out.PublicURL = PublicURL(p.Domain, server)
	}
	return out
}

// previewNumber reads the {number} route parameter.
func previewNumber(ctx contractshttp.Context) (int, bool) {
	n, err := strconv.Atoi(ctx.Request().Route("number"))
	return n, err == nil && n > 0
}

func (c *Controller) Previews(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	list, err := c.service.Previews(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	out := make([]previewJSON, len(list))
	for i, p := range list {
		out[i] = previewToJSON(p)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"previews": out})
}

func (c *Controller) DeployPreview(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	n, okN := previewNumber(ctx)
	if !ok || !okN {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.DeployPreview(ctx.Context(), id, n, domain.TriggerManual)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"deployment": ToJSON(d)})
}

// DeletePreview closes the Preview and removes what it ran.
func (c *Controller) DeletePreview(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	n, okN := previewNumber(ctx)
	if !ok || !okN {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	if err := c.service.ClosePreview(ctx.Context(), id, n); err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
