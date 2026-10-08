package http

import (
	"math"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
)

// figuresJSON is Figures on the wire; tokens is input plus output, as the
// tokens Budget metric counts them. The cost equivalent is rounded to the
// micro-dollars a Run stores, so adding floats leaves no tail.
type figuresJSON struct {
	InputTokens       int64   `json:"input_tokens"`
	CachedInputTokens int64   `json:"cached_input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	Tokens            int64   `json:"tokens"`
	Runs              int64   `json:"runs"`
	RunTimeMS         int64   `json:"run_time_ms"`
	CostEquivalentUSD float64 `json:"cost_equivalent_usd"`
}

func figuresOf(f app.Figures) figuresJSON {
	return figuresJSON{
		InputTokens: f.InputTokens, CachedInputTokens: f.CachedInputTokens, OutputTokens: f.OutputTokens, Tokens: f.Tokens(),
		Runs: f.Runs, RunTimeMS: f.RunTimeMS, CostEquivalentUSD: math.Round(f.CostEquivalentUSD*1e6) / 1e6,
	}
}

type costAgentJSON struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Status string `json:"status"`
}

type agentCostJSON struct {
	Agent costAgentJSON `json:"agent"`
	figuresJSON
}

type projectCostJSON struct {
	Project *Named `json:"project"`
	figuresJSON
}

// costRange reads the optional from and to (RFC 3339, to exclusive); a
// response is the 422 that refused them.
func costRange(ctx contractshttp.Context, guildID uint64) (app.CostRange, contractshttp.Response) {
	q := app.CostRange{GuildID: guildID}
	for _, b := range []struct {
		name string
		into **time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		raw := ctx.Request().Query(b.name, "")
		if raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return q, respond.Invalid(ctx, b.name, "must be an RFC 3339 time")
		}
		t = t.UTC()
		*b.into = &t
	}
	if q.From != nil && q.To != nil && !q.From.Before(*q.To) {
		return q, respond.Invalid(ctx, "to", "must be after from")
	}
	return q, nil
}

// costs answers the Current guild's Costs in the request's range, or the
// response that refused it.
func (c *Controller) costs(ctx contractshttp.Context) (app.CostRange, app.Costs, contractshttp.Response) {
	q, refused := costRange(ctx, c.Guild(ctx))
	if refused != nil {
		return q, app.Costs{}, refused
	}
	out, err := c.service.Costs(ctx.Context(), q, c.visible(ctx))
	if err != nil {
		return q, app.Costs{}, fail(ctx, err)
	}
	return q, out, nil
}

// CostSummary is what the Guild's Runs in the range used, in total.
func (c *Controller) CostSummary(ctx contractshttp.Context) contractshttp.Response {
	q, out, refused := c.costs(ctx)
	if refused != nil {
		return refused
	}
	return ctx.Response().Success().Json(struct {
		From *time.Time `json:"from"`
		To   *time.Time `json:"to"`
		figuresJSON
	}{q.From, q.To, figuresOf(out.Total)})
}

// CostsByAgent is what each Agent's Runs in the range used, most tokens
// first.
func (c *Controller) CostsByAgent(ctx contractshttp.Context) contractshttp.Response {
	_, out, refused := c.costs(ctx)
	if refused != nil {
		return refused
	}
	agents := make([]agentCostJSON, len(out.Agents))
	for i, a := range out.Agents {
		agents[i] = agentCostJSON{
			Agent:       costAgentJSON{ID: a.Agent.ID, Name: a.Agent.Name, Icon: string(a.Agent.Icon), Status: string(a.Agent.Status)},
			figuresJSON: figuresOf(a.Figures),
		}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"agents": agents})
}

// CostsByProject is what the Runs in the range used per Run's Project the
// person may view, most tokens first; a null project is the Runs without
// one.
func (c *Controller) CostsByProject(ctx contractshttp.Context) contractshttp.Response {
	_, out, refused := c.costs(ctx)
	if refused != nil {
		return refused
	}
	projects := make([]projectCostJSON, len(out.Projects))
	for i, p := range out.Projects {
		projects[i] = projectCostJSON{figuresJSON: figuresOf(p.Figures)}
		if p.ProjectID != 0 {
			projects[i].Project = &Named{ID: p.ProjectID, Name: p.ProjectName}
		}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"projects": projects})
}
