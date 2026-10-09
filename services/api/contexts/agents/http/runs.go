package http

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/app/sse"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// How often RunStream looks for new Run events, and how many it sends
// before checking the Run's status.
const (
	runStreamPollEvery = 500 * time.Millisecond
	runStreamBatch     = 500
)

type runAgentJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type runIssueJSON struct {
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

type usageJSON struct {
	InputTokens       int64   `json:"input_tokens"`
	CachedInputTokens int64   `json:"cached_input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	Turns             int64   `json:"turns"`
	CostEquivalentUSD float64 `json:"cost_equivalent_usd"`
	DurationMS        int64   `json:"duration_ms"`
}

type runJSON struct {
	ID               uint64        `json:"id"`
	Agent            runAgentJSON  `json:"agent"`
	Issue            *runIssueJSON `json:"issue"`
	InvocationSource string        `json:"invocation_source"`
	WakeReason       string        `json:"wake_reason"`
	WakeCount        int           `json:"wake_count"`
	Status           string        `json:"status"`
	RequestedBy      *Named        `json:"requested_by"`
	Desktop          *Named        `json:"desktop"`
	RetryOfRunID     *uint64       `json:"retry_of_run_id"`
	Usage            usageJSON     `json:"usage"`
	ExitCode         *int          `json:"exit_code"`
	Error            string        `json:"error"`
	CreatedAt        time.Time     `json:"created_at"`
	StartedAt        *time.Time    `json:"started_at"`
	FinishedAt       *time.Time    `json:"finished_at"`
	CanCancel        bool          `json:"can_cancel"`
	// LimitResetsAt is the Limit reset a limited Run reported.
	LimitResetsAt *time.Time `json:"limit_resets_at"`
	// SubscriptionLimitResetsAt is when the Subscription limit a queued
	// Run waits for resets; null when it waits for none.
	SubscriptionLimitResetsAt *time.Time `json:"subscription_limit_resets_at"`
}

type runEventJSON struct {
	Seq       int64           `json:"seq"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// visible is the request's Visible as the service asks it.
func (c *Controller) visible(ctx contractshttp.Context) app.Visible {
	return func(ids []uint64) ([]uint64, error) { return c.Visible(ctx, ids) }
}

// runsJSON shows the Guild's Runs with their Agents, Issues (when the
// person may view them), who asked, the Desktop running them and whether
// the person may cancel them.
func (c *Controller) runsJSON(ctx contractshttp.Context, guildID uint64, rs []domain.Run) ([]runJSON, error) {
	cx := ctx.Context()
	actor := c.actor(ctx)
	agents := map[uint64]domain.Agent{}
	manages := map[uint64]bool{}
	var memberIDs, desktopIDs []uint64
	for _, r := range rs {
		if _, ok := agents[r.AgentID]; !ok {
			a, err := c.service.Agent(cx, guildID, r.AgentID)
			if err != nil {
				return nil, err
			}
			agents[a.ID] = a
			if manages[a.ID], err = c.service.MayManage(cx, actor, a); err != nil {
				return nil, err
			}
		}
		if r.RequestedByID != 0 {
			memberIDs = append(memberIDs, r.RequestedByID)
		}
		if r.DesktopID != 0 {
			desktopIDs = append(desktopIDs, r.DesktopID)
		}
	}
	members := map[uint64]Named{}
	if len(memberIDs) > 0 {
		ms, err := c.Members(cx, memberIDs)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			members[m.ID] = m
		}
	}
	desktops, err := c.Desktops(cx, desktopIDs)
	if err != nil {
		return nil, err
	}
	issues, err := c.service.IssueBriefs(cx, guildID, rs, c.visible(ctx))
	if err != nil {
		return nil, err
	}
	limits, err := c.service.SubscriptionLimits(cx, rs, agents)
	if err != nil {
		return nil, err
	}
	out := make([]runJSON, len(rs))
	for i, r := range rs {
		a := agents[r.AgentID]
		u := r.Usage
		j := runJSON{
			ID: r.ID, Agent: runAgentJSON{ID: a.ID, Name: a.Name, Icon: string(a.Icon)}, InvocationSource: string(r.InvocationSource),
			WakeReason: string(r.WakeReason), WakeCount: r.WakeCount, Status: string(r.Status), Usage: usageJSON{
				InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, OutputTokens: u.OutputTokens,
				Turns: u.Turns, CostEquivalentUSD: u.CostEquivalentUSD, DurationMS: u.DurationMS,
			},
			ExitCode: r.ExitCode, Error: r.Error, CreatedAt: r.CreatedAt.UTC(), StartedAt: utc(r.StartedAt), FinishedAt: utc(r.FinishedAt),
			CanCancel: !r.Status.Final() && manages[a.ID], LimitResetsAt: utc(r.LimitResetsAt),
		}
		if t, ok := limits[r.ID]; ok {
			j.SubscriptionLimitResetsAt = &t
		}
		if is, ok := issues[r.IssueID]; ok {
			j.Issue = &runIssueJSON{ID: is.ID, Identifier: is.Identifier, Title: is.Title}
		}
		if m, ok := members[r.RequestedByID]; ok {
			j.RequestedBy = &m
		}
		if name, ok := desktops[r.DesktopID]; ok {
			j.Desktop = &Named{ID: r.DesktopID, Name: name}
		}
		if r.RetryOfRunID != 0 {
			id := r.RetryOfRunID
			j.RetryOfRunID = &id
		}
		out[i] = j
	}
	return out, nil
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (c *Controller) runAnswer(ctx contractshttp.Context, status int, r domain.Run) contractshttp.Response {
	out, err := c.runsJSON(ctx, c.Guild(ctx), []domain.Run{r})
	if err != nil {
		return runFail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"run": out[0]})
}

func runFail(ctx contractshttp.Context, err error) contractshttp.Response {
	var (
		rse   *domain.RunStatusError
		block *app.BudgetBlock
	)
	switch {
	case errors.As(err, &block):
		return budgetRefused(ctx, contractshttp.StatusUnprocessableEntity, block)
	case errors.As(err, &rse), errors.As(err, &domain.WakeRefused{}):
		return respond.Error(ctx, contractshttp.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, app.ErrRunNotFound):
		return notFound(ctx)
	}
	return fail(ctx, err)
}

type startRunRequest struct {
	IssueID uint64 `json:"issue_id"`
}

// queuedAnswer is 201 for a new Run, 200 for the queued Run a Wake joined.
func (c *Controller) queuedAnswer(ctx contractshttp.Context, r domain.Run, joined bool) contractshttp.Response {
	if joined {
		return c.runAnswer(ctx, contractshttp.StatusOK, r)
	}
	return c.runAnswer(ctx, contractshttp.StatusCreated, r)
}

// StartRun queues a Run of the Agent on the Issue assigned to it.
func (c *Controller) StartRun(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req startRunRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	r, joined, err := c.service.StartRun(ctx.Context(), c.Guild(ctx), c.actor(ctx), id, req.IssueID, c.visible(ctx))
	if err != nil {
		return runFail(ctx, err)
	}
	return c.queuedAnswer(ctx, r, joined)
}

// RunHeartbeat queues an on_demand Run of the Agent without an Issue
// (Paperclip's /agents/:id/heartbeat/invoke).
func (c *Controller) RunHeartbeat(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	r, joined, err := c.service.RunHeartbeat(ctx.Context(), c.Guild(ctx), c.actor(ctx), id)
	if err != nil {
		return runFail(ctx, err)
	}
	return c.queuedAnswer(ctx, r, joined)
}

// queryID reads an id filter; "" is 0, anything but a number is not ok.
func queryID(ctx contractshttp.Context, key string) (uint64, bool) {
	v := ctx.Request().Query(key, "")
	if v == "" {
		return 0, true
	}
	id, err := strconv.ParseUint(v, 10, 64)
	return id, err == nil
}

// ListRuns answers the Current guild's Runs, newest first, filtered by
// agent, issue and status, at most limit.
func (c *Controller) ListRuns(ctx contractshttp.Context) contractshttp.Response {
	agent, ok := queryID(ctx, "agent")
	if !ok {
		return respond.Invalid(ctx, "agent", "must be an agent id")
	}
	issue, ok := queryID(ctx, "issue")
	if !ok {
		return respond.Invalid(ctx, "issue", "must be an issue id")
	}
	if me := c.agent(ctx); me != 0 {
		agent = me
	}
	guild := c.Guild(ctx)
	rs, err := c.service.Runs(ctx.Context(), guild, app.RunFilter{
		AgentID: agent, IssueID: issue, Status: ctx.Request().Query("status", ""), Limit: ctx.Request().QueryInt("limit", 0),
	})
	if err != nil {
		return runFail(ctx, err)
	}
	out, err := c.runsJSON(ctx, guild, rs)
	if err != nil {
		return runFail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"runs": out})
}

func (c *Controller) ShowRun(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	r, err := c.service.Run(ctx.Context(), c.Guild(ctx), id)
	if err != nil {
		return runFail(ctx, err)
	}
	if me := c.agent(ctx); me != 0 && r.AgentID != me {
		return notFound(ctx)
	}
	return c.runAnswer(ctx, contractshttp.StatusOK, r)
}

// CancelRun ends a queued or running Run.
func (c *Controller) CancelRun(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	r, err := c.service.CancelRun(ctx.Context(), c.Guild(ctx), c.actor(ctx), id)
	if err != nil {
		return runFail(ctx, err)
	}
	return c.runAnswer(ctx, contractshttp.StatusOK, r)
}

// ListRunEvents answers the Run's events after the seq ?after=, by seq.
func (c *Controller) ListRunEvents(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	after, err := strconv.ParseInt(ctx.Request().Query("after", "0"), 10, 64)
	if err != nil {
		return respond.Invalid(ctx, "after", "must be a seq")
	}
	if me := c.agent(ctx); me != 0 {
		r, err := c.service.Run(ctx.Context(), c.Guild(ctx), id)
		if err != nil {
			return runFail(ctx, err)
		}
		if r.AgentID != me {
			return notFound(ctx)
		}
	}
	es, err := c.service.RunEvents(ctx.Context(), c.Guild(ctx), id, after, ctx.Request().QueryInt("limit", 0))
	if err != nil {
		return runFail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"events": runEventsJSON(es)})
}

// runEventsJSON shows Run events as the API answers them.
func runEventsJSON(es []domain.RunEvent) []runEventJSON {
	out := make([]runEventJSON, len(es))
	for i, e := range es {
		payload := json.RawMessage(e.Payload)
		if len(payload) == 0 {
			payload = json.RawMessage("{}")
		}
		out[i] = runEventJSON{Seq: e.Seq, Kind: string(e.Kind), Payload: payload, CreatedAt: e.CreatedAt}
	}
	return out
}

// RunStream sends the Run's stored events, then new ones as the Desktop
// appends them, `status` with the Run's JSON when its status, usage or
// error change, and `end` once it is final. A reconnect resumes after
// Last-Event-ID.
func (c *Controller) RunStream(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	reqCtx := ctx.Request().Origin().Context()
	guild := c.Guild(ctx)
	r, err := c.service.Run(reqCtx, guild, id)
	if err != nil {
		return runFail(ctx, err)
	}
	after, _ := strconv.ParseInt(ctx.Request().Header("Last-Event-ID"), 10, 64)
	stream, ok := sse.Start(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}

	lastStatus := domain.RunStatus("")
	lastUsage := domain.Usage{}
	lastError := ""
	lastPing := time.Now()
	for {
		es, err := c.service.RunEvents(reqCtx, guild, id, after, runStreamBatch)
		if err != nil {
			return nil
		}
		for _, e := range es {
			after = e.Seq
			payload := json.RawMessage(e.Payload)
			if len(payload) == 0 {
				payload = json.RawMessage("{}")
			}
			stream.Event("event", strconv.FormatInt(e.Seq, 10), runEventJSON{Seq: e.Seq, Kind: string(e.Kind), Payload: payload, CreatedAt: e.CreatedAt})
		}
		if len(es) == runStreamBatch {
			continue // more waiting; send before checking the status
		}
		if r, err = c.service.Run(reqCtx, guild, id); err != nil {
			return nil
		}
		if r.Status != lastStatus || r.Usage != lastUsage || r.Error != lastError {
			lastStatus, lastUsage, lastError = r.Status, r.Usage, r.Error
			out, err := c.runsJSON(ctx, guild, []domain.Run{r})
			if err != nil {
				return nil
			}
			stream.Event("status", "", contractshttp.Json{"run": out[0]})
		}
		if r.Status.Final() {
			// Events written just before the final status was saved.
			if more, err := c.service.RunEvents(reqCtx, guild, id, after, runStreamBatch); err == nil && len(more) > 0 {
				continue
			}
			stream.Event("end", "", contractshttp.Json{"status": string(r.Status)})
			return nil
		}
		if time.Since(lastPing) > sse.PingEvery {
			stream.Raw(": ping\n\n")
			lastPing = time.Now()
		}
		select {
		case <-reqCtx.Done():
			return nil
		case <-c.Shutdown:
			return nil
		case <-time.After(runStreamPollEvery):
		}
	}
}

// currentRuns answers the running Run of each of the Agents that has one.
func (c *Controller) currentRuns(cx context.Context, as []domain.Agent) (map[uint64]uint64, error) {
	ids := make([]uint64, len(as))
	for i, a := range as {
		ids[i] = a.ID
	}
	return c.service.RunningRuns(cx, ids)
}
