package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
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
	ID                   uint64          `json:"id"`
	Title                string          `json:"title"`
	Description          string          `json:"description"`
	Project              *namedRef       `json:"project"`
	Goal                 *titledRef      `json:"goal"`
	ParentIssue          *issueRef       `json:"parent_issue"`
	AssigneeAgent        *Agent          `json:"assignee_agent"`
	Priority             string          `json:"priority"`
	Status               string          `json:"status"`
	ConcurrencyPolicy    string          `json:"concurrency_policy"`
	CatchUpPolicy        string          `json:"catch_up_policy"`
	Variables            []variableJSON  `json:"variables"`
	LastTriggeredAt      *time.Time      `json:"last_triggered_at"`
	LatestRevisionID     *uint64         `json:"latest_revision_id"`
	LatestRevisionNumber int             `json:"latest_revision_number"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	Triggers             []triggerJSON   `json:"triggers"`
	LastRun              *routineRunJSON `json:"last_run"`
}

// variableJSON is a Routine variable on the wire, as Paperclip's with its
// names in snake case. Label is null for none.
type variableJSON struct {
	Name     string   `json:"name"`
	Label    *string  `json:"label"`
	Type     string   `json:"type"`
	Default  any      `json:"default_value"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
}

func toVariablesJSON(vars []domain.RoutineVariable) []variableJSON {
	out := make([]variableJSON, len(vars))
	for n, v := range vars {
		out[n] = variableJSON{Name: v.Name, Type: string(v.Type), Default: v.Default, Required: v.Required, Options: v.Options}
		if v.Label != "" {
			out[n].Label = &v.Label
		}
		if out[n].Options == nil {
			out[n].Options = []string{}
		}
	}
	return out
}

// variableRequest is a Routine variable's definition as typed: type text,
// required and no options unless it says otherwise.
type variableRequest struct {
	Name     string   `json:"name"`
	Label    *string  `json:"label"`
	Type     *string  `json:"type"`
	Default  any      `json:"default_value"`
	Required *bool    `json:"required"`
	Options  []string `json:"options"`
}

func variablesOf(reqs []variableRequest) []domain.RoutineVariable {
	out := make([]domain.RoutineVariable, len(reqs))
	for n, v := range reqs {
		out[n] = domain.RoutineVariable{
			Name: v.Name, Label: value(v.Label), Type: domain.TextVariable, Default: v.Default,
			Required: v.Required == nil || *v.Required, Options: v.Options,
		}
		if v.Type != nil {
			out[n].Type = domain.VariableType(*v.Type)
		}
	}
	return out
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
	routineIDs := make([]uint64, len(rs))
	for n, r := range rs {
		routineIDs[n] = r.ID
	}
	triggers, err := c.service.RoutineTriggers(cx, routineIDs)
	if err != nil {
		return nil, err
	}
	last, err := c.service.LastRoutineRuns(cx, routineIDs)
	if err != nil {
		return nil, err
	}
	var lastRuns []domain.RoutineRun
	for _, rr := range last {
		lastRuns = append(lastRuns, rr)
	}
	lastJSON, err := c.routineRunsJSON(ctx, lastRuns)
	if err != nil {
		return nil, err
	}
	lastOf := make(map[uint64]*routineRunJSON, len(lastJSON))
	for n := range lastJSON {
		lastOf[lastJSON[n].Routine.ID] = &lastJSON[n]
	}
	for n, r := range rs {
		out[n] = routineJSON{
			ID: r.ID, Title: r.Title, Description: r.Description, Priority: string(r.Priority), Status: string(r.Status),
			ConcurrencyPolicy: string(r.ConcurrencyPolicy), CatchUpPolicy: string(r.CatchUpPolicy), Variables: toVariablesJSON(r.Variables),
			LastTriggeredAt: r.LastTriggeredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Triggers: []triggerJSON{},
			LastRun: lastOf[r.ID], LatestRevisionID: idOrNull(r.LatestRevisionID), LatestRevisionNumber: r.LatestRevisionNumber,
		}
		for _, t := range triggers[r.ID] {
			out[n].Triggers = append(out[n].Triggers, toTriggerJSON(t))
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
	Title             optional[string]            `json:"title"`
	Description       optional[string]            `json:"description"`
	Priority          optional[string]            `json:"priority"`
	Status            optional[string]            `json:"status"`
	ConcurrencyPolicy optional[string]            `json:"concurrency_policy"`
	CatchUpPolicy     optional[string]            `json:"catch_up_policy"`
	ProjectID         optional[uint64]            `json:"project_id"`
	GoalID            optional[uint64]            `json:"goal_id"`
	ParentIssueID     optional[uint64]            `json:"parent_issue_id"`
	AssigneeAgentID   optional[uint64]            `json:"assignee_agent_id"`
	Variables         optional[[]variableRequest] `json:"variables"`
	BaseRevisionID    optional[uint64]            `json:"base_revision_id"`
}

func (r routineRequest) input() app.RoutineInput {
	return app.RoutineInput{
		Variables: variablesOf(value(r.Variables.ptr())),
		Title:     value(r.Title.ptr()), Description: value(r.Description.ptr()), Priority: value(r.Priority.ptr()), Status: value(r.Status.ptr()),
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
	if r.BaseRevisionID.Set {
		// A null Base revision is one from before revisions.
		p.BaseRevisionID = idOf(r.BaseRevisionID)
	}
	if r.Variables.Set {
		// Null is no definitions: every placeholder takes the default one.
		v := variablesOf(value(r.Variables.ptr()))
		p.Variables = &v
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
	runs, err := c.service.RoutineRuns(ctx.Context(), c.guild(ctx), r.ID, recentRuns, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	recent, err := c.routineRunsJSON(ctx, runs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"routine": struct {
		routineJSON
		RecentRuns []routineRunJSON `json:"recent_runs"`
	}{out[0], recent}})
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

type triggerJSON struct {
	ID              uint64        `json:"id"`
	Kind            string        `json:"kind"`
	Label           string        `json:"label"`
	Enabled         bool          `json:"enabled"`
	CronExpression  *string       `json:"cron_expression"`
	Timezone        *string       `json:"timezone"`
	NextRunAt       *time.Time    `json:"next_run_at"`
	LastFiredAt     *time.Time    `json:"last_fired_at"`
	LastResult      *string       `json:"last_result"`
	SigningMode     *string       `json:"signing_mode"`
	ReplayWindowSec *int          `json:"replay_window_sec"`
	WebhookPath     *string       `json:"webhook_path"`
	LastRotatedAt   *time.Time    `json:"last_rotated_at"`
	LastDelivery    *deliveryJSON `json:"last_delivery"`
}

type deliveryJSON struct {
	Status     string    `json:"status"`
	ReceivedAt time.Time `json:"received_at"`
}

// webhookPath is where a Webhook trigger is fired from outside.
func webhookPath(t domain.RoutineTrigger) string {
	return "/api/routine-triggers/public/" + t.PublicID + "/fire"
}

// toTriggerJSON shows a Routine trigger; what it does not have is null.
// A Webhook trigger's secret is never in it: it is answered only beside
// it, as secret_material, when it is made or rotated.
func toTriggerJSON(t domain.RoutineTrigger) triggerJSON {
	orNull := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	out := triggerJSON{
		ID: t.ID, Kind: string(t.Kind), Label: t.Label, Enabled: t.Enabled,
		CronExpression: orNull(t.CronExpression), Timezone: orNull(t.Timezone),
		NextRunAt: t.NextRunAt, LastFiredAt: t.LastFiredAt, LastResult: orNull(t.LastResult),
	}
	if t.Kind == domain.WebhookTrigger {
		window, path := t.ReplayWindowSec, webhookPath(t)
		out.SigningMode, out.ReplayWindowSec, out.WebhookPath = orNull(string(t.SigningMode)), &window, &path
		out.LastRotatedAt = utcOf(t.LastRotatedAt)
		if d := t.LastDelivery; d != nil {
			out.LastDelivery = &deliveryJSON{Status: string(d.Status), ReceivedAt: d.ReceivedAt.UTC()}
		}
	}
	return out
}

// withSecret answers a Webhook trigger with its secret, the only time the
// secret is shown: when the trigger is made or its secret rotated.
func withSecret(t domain.RoutineTrigger) contractshttp.Json {
	out := contractshttp.Json{"trigger": toTriggerJSON(t)}
	if t.Kind == domain.WebhookTrigger {
		out["secret_material"] = contractshttp.Json{"webhook_path": webhookPath(t), "webhook_secret": t.Secret}
	}
	return out
}

type triggerRequest struct {
	Kind            optional[string] `json:"kind"`
	Label           optional[string] `json:"label"`
	CronExpression  optional[string] `json:"cron_expression"`
	Timezone        optional[string] `json:"timezone"`
	Enabled         optional[bool]   `json:"enabled"`
	SigningMode     optional[string] `json:"signing_mode"`
	ReplayWindowSec optional[int]    `json:"replay_window_sec"`
}

// orEmpty reads an optional string where null is empty: a label taken
// away, a time zone back to UTC.
func orEmpty(o optional[string]) *string {
	if o.Set && o.Value == nil {
		return new(string)
	}
	return o.ptr()
}

// AddTrigger adds a Routine trigger to the {id} Routine.
func (c *Controller) AddTrigger(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req triggerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	t, err := c.service.AddTrigger(ctx.Context(), c.guild(ctx), c.actor(ctx), id, app.TriggerInput{
		Kind: value(req.Kind.ptr()), Label: value(req.Label.ptr()), CronExpression: value(req.CronExpression.ptr()),
		Timezone: value(req.Timezone.ptr()), Enabled: req.Enabled.ptr(),
		SigningMode: value(req.SigningMode.ptr()), ReplayWindowSec: value(req.ReplayWindowSec.ptr()),
	}, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, withSecret(t))
}

// UpdateTrigger changes the {id} Routine trigger.
func (c *Controller) UpdateTrigger(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req triggerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	if req.Kind.Set {
		return respond.Invalid(ctx, "trigger.kind", "a trigger's kind cannot be changed")
	}
	set := domain.TriggerSettings{
		Label: orEmpty(req.Label), CronExpression: req.CronExpression.ptr(), Timezone: orEmpty(req.Timezone), Enabled: req.Enabled.ptr(),
		ReplayWindowSec: req.ReplayWindowSec.ptr(),
	}
	if req.SigningMode.Set {
		mode, err := domain.ParseSigningMode(value(req.SigningMode.ptr()))
		if err != nil {
			return fail(ctx, err)
		}
		set.SigningMode = &mode
	}
	t, err := c.service.ChangeTrigger(ctx.Context(), c.guild(ctx), c.actor(ctx), id, set, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"trigger": toTriggerJSON(t)})
}

// RotateTriggerSecret gives the {id} Webhook trigger a new secret and
// answers it, once.
func (c *Controller) RotateTriggerSecret(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	t, err := c.service.RotateTriggerSecret(ctx.Context(), c.guild(ctx), c.actor(ctx), id, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(withSecret(t))
}

// DeleteTrigger deletes the {id} Routine trigger.
func (c *Controller) DeleteTrigger(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteTrigger(ctx.Context(), c.guild(ctx), c.actor(ctx), id, c.visible(ctx)); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

// recentRuns is how many Routine runs a Routine shows with itself.
const recentRuns = 10

type triggerRef struct {
	ID    uint64 `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type runIssueRef struct {
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

type routineRunJSON struct {
	ID            uint64         `json:"id"`
	Routine       titledRef      `json:"routine"`
	Source        string         `json:"source"`
	Status        string         `json:"status"`
	TriggeredAt   time.Time      `json:"triggered_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	FailureReason *string        `json:"failure_reason"`
	Trigger       *triggerRef    `json:"trigger"`
	Issue         *runIssueRef   `json:"issue"`
	Variables     map[string]any `json:"variables"`
	// RevisionID is the Routine revision it ran, null before revisions.
	RevisionID *uint64 `json:"routine_revision_id"`
}

// routineRunsJSON shows Routine runs with their Routine's title, their
// Routine trigger and their Execution Issue, each kind asked for in one
// go. An Issue the person may not see is left out.
func (c *Controller) routineRunsJSON(ctx contractshttp.Context, rrs []domain.RoutineRun) ([]routineRunJSON, error) {
	out := make([]routineRunJSON, len(rrs))
	if len(rrs) == 0 {
		return out, nil
	}
	cx, guildID := ctx.Context(), c.guild(ctx)
	var routineIDs, issueIDs []uint64
	for _, rr := range rrs {
		routineIDs = append(routineIDs, rr.RoutineID)
		if rr.LinkedIssueID != 0 {
			issueIDs = append(issueIDs, rr.LinkedIssueID)
		}
	}
	titles, err := c.service.RoutineTitles(cx, guildID, routineIDs)
	if err != nil {
		return nil, err
	}
	ts, err := c.service.RoutineTriggers(cx, routineIDs)
	if err != nil {
		return nil, err
	}
	triggers := map[uint64]triggerRef{}
	for _, list := range ts {
		for _, t := range list {
			triggers[t.ID] = triggerRef{ID: t.ID, Kind: string(t.Kind), Label: t.Label}
		}
	}
	issues := map[uint64]runIssueRef{}
	if len(issueIDs) > 0 {
		prefix, err := c.service.IssuePrefix(cx, guildID)
		if err != nil {
			return nil, err
		}
		is, err := c.service.VisibleIssues(cx, issueIDs, c.visible(ctx))
		if err != nil {
			return nil, err
		}
		for _, i := range is {
			issues[i.ID] = runIssueRef{ID: i.ID, Identifier: domain.Identifier(prefix, i.Number), Title: i.Title, Status: string(i.Status)}
		}
	}
	for n, rr := range rrs {
		out[n] = routineRunJSON{
			ID: rr.ID, Routine: titledRef{ID: rr.RoutineID, Title: titles[rr.RoutineID]}, Source: string(rr.Source), Status: string(rr.Status),
			TriggeredAt: rr.TriggeredAt.UTC(), CompletedAt: utcOf(rr.CompletedAt), Variables: rr.Variables,
			RevisionID: idOrNull(rr.RoutineRevisionID),
		}
		if rr.FailureReason != "" {
			reason := rr.FailureReason
			out[n].FailureReason = &reason
		}
		if t, ok := triggers[rr.TriggerID]; ok {
			out[n].Trigger = &t
		}
		if i, ok := issues[rr.LinkedIssueID]; ok {
			out[n].Issue = &i
		}
	}
	return out, nil
}

func utcOf(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

type runRequest struct {
	TriggerID optional[uint64] `json:"trigger_id"`
	Variables map[string]any   `json:"variables"`
}

// RunRoutine runs the {id} Routine now: manual, or api when it names one
// of its api Routine triggers.
func (c *Controller) RunRoutine(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req runRequest
	// The body, {trigger_id, variables}, is optional.
	if ctx.Request().Origin().ContentLength != 0 {
		if err := ctx.Request().Bind(&req); err != nil {
			return respond.BadBody(ctx)
		}
	}
	run := app.RunRequest{Source: domain.ManualSource, Actor: c.actor(ctx), Variables: req.Variables}
	if t := value(idOf(req.TriggerID)); t != 0 {
		run.Source, run.TriggerID = domain.APISource, t
	}
	rr, err := c.service.RunRoutine(ctx.Context(), c.guild(ctx), id, run)
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.routineRunsJSON(ctx, []domain.RoutineRun{rr})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusAccepted, contractshttp.Json{"routine_run": out[0]})
}

// ListRoutineRuns answers the {id} Routine's Routine runs, or without an
// {id} the Current guild's across the Routines the request may view,
// newest first; ?limit= is at most 200, 50 by default.
func (c *Controller) ListRoutineRuns(ctx contractshttp.Context) contractshttp.Response {
	var id uint64
	if ctx.Request().Route("id") != "" {
		var ok bool
		if id, ok = routeID(ctx); !ok {
			return notFound(ctx)
		}
	}
	limit := 50
	if v := ctx.Request().Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			return respond.Invalid(ctx, "limit", "limit must be a number from 1 to 200")
		}
		limit = n
	}
	rrs, err := c.service.RoutineRuns(ctx.Context(), c.guild(ctx), id, limit, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.routineRunsJSON(ctx, rrs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"routine_runs": out})
}

// maxDelivery is the largest Webhook delivery body read, in bytes.
const maxDelivery = 1 << 20

// publicRunJSON is a Routine run as its Webhook delivery's sender sees
// it: ids only, since the sender is not a Member of the Guild.
type publicRunJSON struct {
	ID                 uint64    `json:"id"`
	RoutineID          uint64    `json:"routine_id"`
	TriggerID          uint64    `json:"trigger_id"`
	Source             string    `json:"source"`
	Status             string    `json:"status"`
	TriggeredAt        time.Time `json:"triggered_at"`
	IssueID            *uint64   `json:"issue_id"`
	CoalescedIntoRunID *uint64   `json:"coalesced_into_run_id"`
}

func idOrNull(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

// FireWebhookTrigger is a Webhook delivery to the {public_id} Webhook
// trigger. It carries no Session; its Signing mode is the authentication,
// and a refused one answers 401 without saying which check failed.
func (c *Controller) FireWebhookTrigger(ctx contractshttp.Context) contractshttp.Response {
	req := ctx.Request().Origin()
	if mt, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type")); mt != "application/json" {
		return respond.Error(ctx, contractshttp.StatusUnsupportedMediaType, "Send the webhook payload with Content-Type: application/json")
	}
	const notObject = "Webhook payload must be a JSON object"
	// Goravel decodes a JSON body into an object before any handler runs
	// and puts the raw bytes back only when that worked, so a body it
	// leaves unreadable was not a JSON object.
	body, err := io.ReadAll(io.LimitReader(req.Body, maxDelivery+1))
	if err != nil {
		return respond.Error(ctx, contractshttp.StatusBadRequest, notObject)
	}
	if len(body) > maxDelivery {
		return respond.Error(ctx, contractshttp.StatusRequestEntityTooLarge, "payload too large")
	}
	// An empty body is the empty object, as Paperclip's JSON parser reads it.
	if len(bytes.TrimSpace(body)) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil || payload == nil {
			return respond.Error(ctx, contractshttp.StatusBadRequest, notObject)
		}
	}
	key := req.Header.Get("Idempotency-Key")
	if key == "" {
		key = req.Header.Get("X-GitHub-Delivery")
	}
	h := domain.DeliveryHeaders{
		Authorization: req.Header.Get("Authorization"), Signature: req.Header.Get("X-Bakery-Signature"),
		HubSignature256: req.Header.Get("X-Hub-Signature-256"), Timestamp: req.Header.Get("X-Bakery-Timestamp"), IdempotencyKey: key,
	}
	rr, err := c.service.FireWebhookTrigger(ctx.Context(), ctx.Request().Route("public_id"), h, body)
	switch {
	case errors.Is(err, domain.ErrBadCredentials), errors.Is(err, domain.ErrOutsideReplayWindow):
		return ctx.Response().Json(contractshttp.StatusUnauthorized, contractshttp.Json{"error": "unauthorized"})
	case err != nil:
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusAccepted, contractshttp.Json{"routine_run": publicRunJSON{
		ID: rr.ID, RoutineID: rr.RoutineID, TriggerID: rr.TriggerID, Source: string(rr.Source), Status: string(rr.Status),
		TriggeredAt: rr.TriggeredAt.UTC(), IssueID: idOrNull(rr.LinkedIssueID), CoalescedIntoRunID: idOrNull(rr.CoalescedIntoRunID),
	}})
}

// revisionAuthorJSON is who made a Routine revision: a Member or an
// Agent, by kind.
type revisionAuthorJSON struct {
	Kind string `json:"kind"`
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

type routineRevisionJSON struct {
	ID                     uint64                 `json:"id"`
	RoutineID              uint64                 `json:"routine_id"`
	RevisionNumber         int                    `json:"revision_number"`
	Title                  string                 `json:"title"`
	Description            string                 `json:"description"`
	Snapshot               domain.RoutineSnapshot `json:"snapshot"`
	ChangeSummary          *string                `json:"change_summary"`
	RestoredFromRevisionID *uint64                `json:"restored_from_revision_id"`
	Author                 *revisionAuthorJSON    `json:"author"`
	CreatedAt              time.Time              `json:"created_at"`
}

// routineRevisionsJSON shows Routine revisions with their authors' names,
// asked for in one go.
func (c *Controller) routineRevisionsJSON(ctx contractshttp.Context, revs []domain.RoutineRevision) ([]routineRevisionJSON, error) {
	authors := make([]domain.Actor, len(revs))
	for n, r := range revs {
		authors[n] = r.Author
	}
	ns, err := c.actorNames(ctx, authors)
	if err != nil {
		return nil, err
	}
	out := make([]routineRevisionJSON, len(revs))
	for n, r := range revs {
		out[n] = routineRevisionJSON{
			ID: r.ID, RoutineID: r.RoutineID, RevisionNumber: r.Number, Title: r.Title, Description: r.Description, Snapshot: r.Snapshot,
			RestoredFromRevisionID: idOrNull(r.RestoredFromID), CreatedAt: r.CreatedAt,
		}
		if r.ChangeSummary != "" {
			summary := r.ChangeSummary
			out[n].ChangeSummary = &summary
		}
		if m := ns.member(r.Author); m != nil {
			out[n].Author = &revisionAuthorJSON{Kind: "member", ID: m.ID, Name: m.Name}
		} else if a := ns.agent(r.Author); a != nil {
			out[n].Author = &revisionAuthorJSON{Kind: "agent", ID: a.ID, Name: a.Name, Icon: a.Icon}
		}
	}
	return out, nil
}

// ListRoutineRevisions answers the Routine's revisions, newest first, at
// most app.MaxRoutineRevisions.
func (c *Controller) ListRoutineRevisions(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	revs, err := c.service.RoutineRevisions(ctx.Context(), c.guild(ctx), id, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.routineRevisionsJSON(ctx, revs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"revisions": out})
}
