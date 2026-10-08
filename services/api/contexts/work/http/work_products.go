package http

import (
	"net/url"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"

	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// workProductJSON is a Work product: a Pull request (status open, merged
// or closed) or a Preview's link (status deploying, ready, failed or
// removed). external_id is the Pull request's number for both.
type workProductJSON struct {
	ID             uint64    `json:"id"`
	Type           string    `json:"type"`
	ApplicationID  uint64    `json:"application_id"`
	Provider       string    `json:"provider"`
	ExternalID     string    `json:"external_id"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	Status         string    `json:"status"`
	CreatedByRun   *uint64   `json:"created_by_run_id"`
	CreatedBy      *Member   `json:"created_by"`
	CreatedByAgent *Agent    `json:"created_by_agent"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (c *Controller) workProductsJSON(ctx contractshttp.Context, ws []domain.WorkProduct) ([]workProductJSON, error) {
	actors := make([]domain.Actor, len(ws))
	for n, w := range ws {
		actors[n] = w.CreatedBy
	}
	names, err := c.actorNames(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := make([]workProductJSON, len(ws))
	for n, w := range ws {
		out[n] = workProductJSON{
			ID: w.ID, Type: string(w.Type), ApplicationID: w.ApplicationID, Provider: w.Provider, ExternalID: w.ExternalID,
			Title: w.Title, URL: w.URL, Status: string(w.Status), CreatedBy: names.member(w.CreatedBy),
			CreatedByAgent: names.agent(w.CreatedBy), CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
		}
		if w.CreatedBy.RunID != 0 {
			run := w.CreatedBy.RunID
			out[n].CreatedByRun = &run
		}
	}
	return out, nil
}

// OpenPullRequest opens the Issue's Pull request from its Agent branch:
// 201 with the new Work product, 200 with the one it already had open.
func (c *Controller) OpenPullRequest(ctx contractshttp.Context) contractshttp.Response {
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	issueURL := func(identifier string) string { return c.DashboardURL() + "/#/issues/" + url.PathEscape(identifier) }
	w, created, err := c.service.OpenPullRequest(ctx.Context(), c.guild(ctx), c.actor(ctx), ctx.Request().Route("id"), app.PullRequestInput{Title: req.Title, Body: req.Body}, issueURL, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.workProductsJSON(ctx, []domain.WorkProduct{w})
	if err != nil {
		return fail(ctx, err)
	}
	status := contractshttp.StatusOK
	if created {
		status = contractshttp.StatusCreated
	}
	return ctx.Response().Json(status, contractshttp.Json{"work_product": out[0]})
}
