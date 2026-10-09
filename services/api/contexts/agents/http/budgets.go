package http

import (
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

type budgetScopeJSON struct {
	Type string `json:"type"`
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// budgetJSON is a Budget on the wire; a lifetime window has no start or
// end.
type budgetJSON struct {
	ID          uint64          `json:"id"`
	Scope       budgetScopeJSON `json:"scope"`
	Metric      string          `json:"metric"`
	Window      string          `json:"window"`
	Amount      int64           `json:"amount"`
	WarnPercent int             `json:"warn_percent"`
	HardStop    bool            `json:"hard_stop"`
	Notify      bool            `json:"notify"`
	Observed    int64           `json:"observed"`
	Status      string          `json:"status"`
	WindowStart *time.Time      `json:"window_start"`
	WindowEnd   *time.Time      `json:"window_end"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func budgetOf(b app.BudgetSummary) budgetJSON {
	out := budgetJSON{
		ID: b.ID, Scope: budgetScopeJSON{Type: string(b.Scope), ID: b.ScopeID, Name: b.ScopeName},
		Metric: string(b.Metric), Window: string(b.Window), Amount: b.Amount, WarnPercent: b.WarnPercent,
		HardStop: b.HardStop, Notify: b.Notify, Observed: b.Observed, Status: string(b.Status), UpdatedAt: b.UpdatedAt,
	}
	if !b.WindowStart.IsZero() {
		out.WindowStart, out.WindowEnd = &b.WindowStart, &b.WindowEnd
	}
	return out
}

// incidentJSON is a Budget incident on the wire; a lifetime window has no
// start or end.
type incidentJSON struct {
	ID          uint64          `json:"id"`
	BudgetID    uint64          `json:"budget_id"`
	Scope       budgetScopeJSON `json:"scope"`
	Metric      string          `json:"metric"`
	Window      string          `json:"window"`
	Threshold   string          `json:"threshold"`
	Amount      int64           `json:"amount"`
	Observed    int64           `json:"observed"`
	Status      string          `json:"status"`
	ApprovalID  *uint64         `json:"approval_id"`
	WindowStart *time.Time      `json:"window_start"`
	WindowEnd   *time.Time      `json:"window_end"`
	CreatedAt   time.Time       `json:"created_at"`
	ResolvedAt  *time.Time      `json:"resolved_at"`
}

func incidentOf(i app.IncidentSummary) incidentJSON {
	out := incidentJSON{
		ID: i.ID, BudgetID: i.BudgetID, Scope: budgetScopeJSON{Type: string(i.Scope), ID: i.ScopeID, Name: i.ScopeName},
		Metric: string(i.Metric), Window: string(i.Window), Threshold: string(i.Threshold), Amount: i.Amount,
		Observed: i.Observed, Status: string(i.Status), CreatedAt: i.CreatedAt, ResolvedAt: i.ResolvedAt,
	}
	if i.ApprovalID != 0 {
		out.ApprovalID = &i.ApprovalID
	}
	if !i.WindowStart.IsZero() {
		out.WindowStart, out.WindowEnd = &i.WindowStart, &i.WindowEnd
	}
	return out
}

// budgetRefused answers a Wake in a stopped scope: 422 (409 for a
// Desktop's claim) with the reason and the Budget scope.
func budgetRefused(ctx contractshttp.Context, status int, b *app.BudgetBlock) contractshttp.Response {
	return ctx.Response().Json(status, contractshttp.Json{
		"message": b.Reason, "scope": budgetScopeJSON{Type: string(b.Scope), ID: b.ScopeID, Name: b.ScopeName},
	})
}

// BudgetOverview answers the Current guild's Budgets with their Observed
// amounts, open Budget incidents and stopped scopes.
func (c *Controller) BudgetOverview(ctx contractshttp.Context) contractshttp.Response {
	o, err := c.service.BudgetOverview(ctx.Context(), c.Guild(ctx), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	budgets := make([]budgetJSON, len(o.Budgets))
	for i, b := range o.Budgets {
		budgets[i] = budgetOf(b)
	}
	incidents := make([]incidentJSON, len(o.Incidents))
	for i, in := range o.Incidents {
		incidents[i] = incidentOf(in)
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"budgets": budgets, "incidents": incidents, "paused_agents": o.PausedAgents, "stopped_projects": o.StoppedProjects,
	})
}

type budgetRequest struct {
	ScopeType   string  `json:"scope_type"`
	ScopeID     *uint64 `json:"scope_id"`
	Metric      string  `json:"metric"`
	Window      string  `json:"window"`
	Amount      *int64  `json:"amount"`
	WarnPercent *int    `json:"warn_percent"`
	HardStop    *bool   `json:"hard_stop"`
	Notify      *bool   `json:"notify"`
}

// SetBudget creates or changes the Current guild's one Budget for the
// scope, metric and window. A guild scope may leave its id out; the
// Warning defaults to 80 percent, Hard stop and notify to on.
func (c *Controller) SetBudget(ctx contractshttp.Context) contractshttp.Response {
	var req budgetRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	guild := c.Guild(ctx)
	scope, err := domain.ParseBudgetScope(req.ScopeType)
	if err != nil {
		return fail(ctx, err)
	}
	metric, err := domain.ParseBudgetMetric(req.Metric)
	if err != nil {
		return fail(ctx, err)
	}
	in := app.BudgetInput{Scope: scope, Metric: metric, Window: req.Window,
		BudgetTerms: domain.BudgetTerms{WarnPercent: domain.DefaultWarnPercent, HardStop: true, Notify: true}}
	switch {
	case req.ScopeID != nil:
		in.ScopeID = *req.ScopeID
	case scope == domain.GuildScope:
		in.ScopeID = guild
	default:
		return respond.Invalid(ctx, "scope_id", "is required")
	}
	if req.Amount == nil {
		return respond.Invalid(ctx, "amount", "is required")
	}
	in.Amount = *req.Amount
	if req.WarnPercent != nil {
		in.WarnPercent = *req.WarnPercent
	}
	if req.HardStop != nil {
		in.HardStop = *req.HardStop
	}
	if req.Notify != nil {
		in.Notify = *req.Notify
	}
	b, err := c.service.SetBudget(ctx.Context(), guild, c.Member(ctx), in, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"budget": budgetOf(b)})
}

type resolveIncidentRequest struct {
	Action       string `json:"action"`
	Amount       *int64 `json:"amount"`
	DecisionNote string `json:"decision_note"`
}

// ResolveBudgetIncident raises the Budget of one of the Current guild's
// open hard Budget incidents and resumes its scope, or keeps it paused.
func (c *Controller) ResolveBudgetIncident(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req resolveIncidentRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	action, err := app.ParseIncidentAction(req.Action)
	if err != nil {
		return fail(ctx, err)
	}
	var amount int64
	if action == app.RaiseBudgetAndResume {
		if req.Amount == nil {
			return respond.Invalid(ctx, "amount", "is required")
		}
		amount = *req.Amount
	}
	i, err := c.service.ResolveBudgetIncident(ctx.Context(), c.Guild(ctx), c.Member(ctx), id, action, amount, req.DecisionNote, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"incident": incidentOf(i)})
}
