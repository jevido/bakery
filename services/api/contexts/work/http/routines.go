package http

import (
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type namedRef struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type titledRef struct {
	ID    uint64 `json:"id"`
	Title string `json:"title"`
}

type issueRef struct {
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

type routineJSON struct {
	ID                uint64     `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Project           *namedRef  `json:"project"`
	Goal              *titledRef `json:"goal"`
	ParentIssue       *issueRef  `json:"parent_issue"`
	AssigneeAgent     *Agent     `json:"assignee_agent"`
	Priority          string     `json:"priority"`
	Status            string     `json:"status"`
	ConcurrencyPolicy string     `json:"concurrency_policy"`
	CatchUpPolicy     string     `json:"catch_up_policy"`
	LastTriggeredAt   *time.Time `json:"last_triggered_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	Triggers          []any      `json:"triggers"`
	LastRun           any        `json:"last_run"`
}

// routinesJSON shows Routines with the names of their Projects, Goals,
// parent Issues and Agent assignees, each kind asked for in one go. A
// parent Issue the person may not see is left out.
func (c *Controller) routinesJSON(ctx contractshttp.Context, rs []domain.Routine) ([]routineJSON, error) {
	out := make([]routineJSON, len(rs))
	if len(rs) == 0 {
		return out, nil
	}
	cx, guildID := ctx.Context(), c.guild(ctx)
	var projectIDs, parentIDs, agentIDs []uint64
	for _, r := range rs {
		if r.ProjectID != 0 {
			projectIDs = append(projectIDs, r.ProjectID)
		}
		if r.ParentIssueID != 0 {
			parentIDs = append(parentIDs, r.ParentIssueID)
		}
		if r.AssigneeAgentID != 0 {
			agentIDs = append(agentIDs, r.AssigneeAgentID)
		}
	}
	projects, err := c.service.ProjectNames(cx, guildID, projectIDs)
	if err != nil {
		return nil, err
	}
	gs, err := c.service.Goals(cx, guildID)
	if err != nil {
		return nil, err
	}
	goals := make(map[uint64]string, len(gs))
	for _, g := range gs {
		goals[g.ID] = g.Title
	}
	parents := map[uint64]issueRef{}
	if len(parentIDs) > 0 {
		prefix, err := c.service.IssuePrefix(cx, guildID)
		if err != nil {
			return nil, err
		}
		is, err := c.service.VisibleIssues(cx, parentIDs, c.visible(ctx))
		if err != nil {
			return nil, err
		}
		for _, i := range is {
			parents[i.ID] = issueRef{ID: i.ID, Identifier: domain.Identifier(prefix, i.Number), Title: i.Title}
		}
	}
	agents, err := c.service.AssigneeAgents(cx, guildID, agentIDs)
	if err != nil {
		return nil, err
	}
	for n, r := range rs {
		out[n] = routineJSON{
			ID: r.ID, Title: r.Title, Description: r.Description, Priority: string(r.Priority), Status: string(r.Status),
			ConcurrencyPolicy: string(r.ConcurrencyPolicy), CatchUpPolicy: string(r.CatchUpPolicy),
			LastTriggeredAt: r.LastTriggeredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Triggers: []any{},
		}
		if name, ok := projects[r.ProjectID]; ok {
			out[n].Project = &namedRef{ID: r.ProjectID, Name: name}
		}
		if title, ok := goals[r.GoalID]; ok {
			out[n].Goal = &titledRef{ID: r.GoalID, Title: title}
		}
		if p, ok := parents[r.ParentIssueID]; ok {
			out[n].ParentIssue = &p
		}
		if a, ok := agents[r.AssigneeAgentID]; ok {
			out[n].AssigneeAgent = &Agent{ID: r.AssigneeAgentID, Name: a.Name, Icon: a.Icon}
		}
	}
	return out, nil
}

type routineRequest struct {
	Title             optional[string] `json:"title"`
	Description       optional[string] `json:"description"`
	Priority          optional[string] `json:"priority"`
	Status            optional[string] `json:"status"`
	ConcurrencyPolicy optional[string] `json:"concurrency_policy"`
	CatchUpPolicy     optional[string] `json:"catch_up_policy"`
	ProjectID         optional[uint64] `json:"project_id"`
	GoalID            optional[uint64] `json:"goal_id"`
	ParentIssueID     optional[uint64] `json:"parent_issue_id"`
	AssigneeAgentID   optional[uint64] `json:"assignee_agent_id"`
}

func (r routineRequest) input() app.RoutineInput {
	return app.RoutineInput{
		Title: value(r.Title.ptr()), Description: value(r.Description.ptr()), Priority: value(r.Priority.ptr()), Status: value(r.Status.ptr()),
		ConcurrencyPolicy: value(r.ConcurrencyPolicy.ptr()), CatchUpPolicy: value(r.CatchUpPolicy.ptr()),
		ProjectID: value(idOf(r.ProjectID)), GoalID: value(idOf(r.GoalID)), ParentIssueID: value(idOf(r.ParentIssueID)), AssigneeAgentID: value(idOf(r.AssigneeAgentID)),
	}
}

func (r routineRequest) patch() app.RoutinePatch {
	p := app.RoutinePatch{
		Title: r.Title.ptr(), Description: r.Description.ptr(), Priority: r.Priority.ptr(), Status: r.Status.ptr(),
		ConcurrencyPolicy: r.ConcurrencyPolicy.ptr(), CatchUpPolicy: r.CatchUpPolicy.ptr(),
		ProjectID: idOf(r.ProjectID), GoalID: idOf(r.GoalID), ParentIssueID: idOf(r.ParentIssueID), AssigneeAgentID: idOf(r.AssigneeAgentID),
	}
	if r.Description.Set && p.Description == nil {
		// A null description is an empty one.
		p.Description = new(string)
	}
	return p
}

func (c *Controller) oneRoutine(ctx contractshttp.Context, status int, r domain.Routine, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.routinesJSON(ctx, []domain.Routine{r})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"routine": struct {
		routineJSON
		RecentRuns []any `json:"recent_runs"`
	}{out[0], []any{}}})
}

// routineFilter reads ?project_id=, ?assignee_agent_id= (each an id, or
// none) and ?status=.
func routineFilter(ctx contractshttp.Context) (app.RoutineFilter, contractshttp.Response) {
	var f app.RoutineFilter
	for _, q := range []struct {
		key string
		to  **uint64
	}{{"project_id", &f.ProjectID}, {"assignee_agent_id", &f.AssigneeAgentID}} {
		v := ctx.Request().Query(q.key)
		if v == "" {
			continue
		}
		var id uint64
		if v != "none" {
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil || n == 0 {
				return f, respond.Invalid(ctx, q.key, q.key+" must be an id or none")
			}
			id = n
		}
		*q.to = &id
	}
	if v := ctx.Request().Query("status"); v != "" {
		st, err := domain.ParseRoutineStatus(v)
		if err != nil {
			return f, fail(ctx, err)
		}
		f.Status = &st
	}
	return f, nil
}

// ListRoutines answers the Current guild's Routines the request may view,
// newest first.
func (c *Controller) ListRoutines(ctx contractshttp.Context) contractshttp.Response {
	f, bad := routineFilter(ctx)
	if bad != nil {
		return bad
	}
	rs, err := c.service.Routines(ctx.Context(), c.guild(ctx), f, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.routinesJSON(ctx, rs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"routines": out})
}

func (c *Controller) CreateRoutine(ctx contractshttp.Context) contractshttp.Response {
	var req routineRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	r, err := c.service.CreateRoutine(ctx.Context(), c.guild(ctx), c.actor(ctx), req.input(), c.visible(ctx))
	return c.oneRoutine(ctx, contractshttp.StatusCreated, r, err)
}

func (c *Controller) ShowRoutine(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	r, err := c.service.Routine(ctx.Context(), c.guild(ctx), id, c.visible(ctx))
	return c.oneRoutine(ctx, contractshttp.StatusOK, r, err)
}

func (c *Controller) UpdateRoutine(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req routineRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	r, err := c.service.ChangeRoutine(ctx.Context(), c.guild(ctx), c.actor(ctx), id, req.patch(), c.visible(ctx))
	return c.oneRoutine(ctx, contractshttp.StatusOK, r, err)
}
