package http

import (
	"encoding/json"
	"errors"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/app/sse"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// The most Run events one report carries, and the largest payload of one.
const (
	maxReportEvents = 500
	maxEventPayload = 256 << 10
)

// How often the Desktop's stream looks for changes, and how often it
// tells identity the Desktop is still connected.
const (
	desktopPollEvery = time.Second
	desktopSeeEvery  = time.Minute
)

// DesktopController serves the routes the Desktop app's Runner uses with
// its Desktop key, across every Guild of its Member.
type DesktopController struct {
	service *app.Service
	// Desktop is the Member and Desktop of the request's Desktop key
	// (identity.DesktopOf).
	Desktop func(ctx contractshttp.Context) (memberID, desktopID uint64, ok bool)
	// SeeDesktop records that the Desktop is still connected
	// (identity.SeeDesktop).
	SeeDesktop func(ctx contractshttp.Context, desktopID uint64) error
	// Shutdown closes when the API stops, ending open streams.
	Shutdown <-chan struct{}
}

func NewDesktopController(service *app.Service) *DesktopController {
	return &DesktopController{service: service}
}

func (c *DesktopController) desktop(ctx contractshttp.Context) app.Desktop {
	m, d, _ := c.Desktop(ctx)
	return app.Desktop{ID: d, MemberID: m}
}

type namedJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type desktopRunJSON struct {
	ID             uint64        `json:"id"`
	Status         string        `json:"status"`
	Guild          namedJSON     `json:"guild"`
	Agent          runAgentJSON  `json:"agent"`
	Issue          *runIssueJSON `json:"issue"`
	Source         string        `json:"invocation_source"`
	WakeReason     string        `json:"wake_reason"`
	WakeCount      int           `json:"wake_count"`
	Prompt         string        `json:"prompt"`
	RetryOfRunID   *uint64       `json:"retry_of_run_id"`
	SessionID      string        `json:"session_id"`
	NextSeq        int64         `json:"next_seq"`
	CreatedAt      time.Time     `json:"created_at"`
	StartedAt      *time.Time    `json:"started_at"`
	LeaseExpiresAt *time.Time    `json:"lease_expires_at"`
	// RunKey is only in the claim's answer, the one time it is shown.
	RunKey string `json:"run_key,omitempty"`
	// Workspace is only filled in the claim's answer; null otherwise.
	Workspace *workspaceJSON `json:"workspace"`
}

type workspaceJSON struct {
	Application namedJSON `json:"application"`
	Repository  string    `json:"repository"`
	BaseBranch  string    `json:"base_branch"`
	Branch      string    `json:"branch"`
}

func desktopRunOf(q app.QueuedRun) desktopRunJSON {
	r := q.Run
	j := desktopRunJSON{
		ID: r.ID, Status: string(r.Status), Guild: namedJSON{ID: r.GuildID, Name: q.GuildName},
		Agent:  runAgentJSON{ID: q.Agent.ID, Name: q.Agent.Name, Icon: string(q.Agent.Icon)},
		Source: string(r.InvocationSource), WakeReason: string(r.WakeReason), WakeCount: r.WakeCount,
		Prompt: r.Prompt, SessionID: r.SessionID, NextSeq: r.NextSeq, CreatedAt: r.CreatedAt.UTC(),
		StartedAt: utc(r.StartedAt), LeaseExpiresAt: utc(r.LeaseExpiresAt), RunKey: q.RunKey,
	}
	if q.HasIssue {
		j.Issue = &runIssueJSON{ID: q.Issue.ID, Identifier: q.Issue.Identifier, Title: q.Issue.Title}
	}
	if w := q.Workspace; w != nil {
		j.Workspace = &workspaceJSON{
			Application: namedJSON{ID: w.ApplicationID, Name: w.ApplicationName},
			Repository:  w.Repository, BaseBranch: w.BaseBranch, Branch: w.Branch,
		}
	}
	if r.RetryOfRunID != 0 {
		id := r.RetryOfRunID
		j.RetryOfRunID = &id
	}
	return j
}

func desktopRunsOf(qs []app.QueuedRun) []desktopRunJSON {
	out := make([]desktopRunJSON, len(qs))
	for i, q := range qs {
		out[i] = desktopRunOf(q)
	}
	return out
}

// desktopFail answers what the Runner may meet: 404 for another person's
// Run, 403 for one another Desktop claimed, 409 for a Run that moved on or
// an Agent that cannot take it, 422 for a gap in the seqs.
func desktopFail(ctx contractshttp.Context, err error) contractshttp.Response {
	var (
		rse *domain.RunStatusError
		se  *domain.StatusError
		qe  *domain.SeqError
		fe  *domain.FieldError
	)
	switch {
	case errors.Is(err, app.ErrRunNotFound):
		return notFound(ctx)
	case errors.Is(err, app.ErrOtherDesktop):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.Is(err, app.ErrAgentBusy), errors.As(err, &rse), errors.As(err, &se):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case errors.As(err, &qe):
		return ctx.Response().Json(contractshttp.StatusUnprocessableEntity, contractshttp.Json{
			"message": err.Error(), "errors": map[string]string{"events": err.Error()}, "expected_seq": qe.Expected,
		})
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	}
	return respond.ServerError(ctx, err)
}

// ListRuns answers the Runs waiting for the Desktop's Member and the
// running ones this Desktop holds, oldest first.
func (c *DesktopController) ListRuns(ctx contractshttp.Context) contractshttp.Response {
	qs, err := c.service.DesktopRuns(ctx.Context(), c.desktop(ctx))
	if err != nil {
		return desktopFail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"runs": desktopRunsOf(qs)})
}

// StreamRuns sends `runs` with the same list as ListRuns whenever it
// changes, and `cancel` {run_id} when a Run this Desktop holds was
// cancelled, until the Desktop leaves or the API stops.
func (c *DesktopController) StreamRuns(ctx contractshttp.Context) contractshttp.Response {
	reqCtx := ctx.Request().Origin().Context()
	d := c.desktop(ctx)
	stream, ok := sse.Start(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}
	var last string
	held := map[uint64]bool{}
	lastPing, lastSeen := time.Now(), time.Now()
	for {
		qs, err := c.service.DesktopRuns(reqCtx, d)
		if err != nil {
			return nil
		}
		now := map[uint64]bool{}
		for _, q := range qs {
			if q.Run.Status == domain.RunRunning {
				now[q.Run.ID] = true
			}
		}
		for id := range held {
			if now[id] {
				continue
			}
			if r, err := c.service.DesktopRun(reqCtx, d, id); err == nil && r.Status == domain.RunCancelled {
				stream.Event("cancel", "", contractshttp.Json{"run_id": id})
			}
		}
		held = now
		body, _ := json.Marshal(desktopRunsOf(qs))
		if string(body) != last {
			last = string(body)
			stream.Event("runs", "", contractshttp.Json{"runs": json.RawMessage(body)})
			lastPing = time.Now()
		}
		if time.Since(lastPing) > sse.PingEvery {
			stream.Raw(": ping\n\n")
			lastPing = time.Now()
		}
		if time.Since(lastSeen) > desktopSeeEvery {
			_ = c.SeeDesktop(ctx, d.ID)
			lastSeen = time.Now()
		}
		select {
		case <-reqCtx.Done():
			return nil
		case <-c.Shutdown:
			return nil
		case <-time.After(desktopPollEvery):
		}
	}
}

// ClaimRun takes a queued Run for this Desktop and answers it, prompt
// and all.
func (c *DesktopController) ClaimRun(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	q, err := c.service.ClaimRun(ctx.Context(), c.desktop(ctx), id)
	if err != nil {
		return desktopFail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"run": desktopRunOf(q)})
}

type reportedEvent struct {
	Seq     int64           `json:"seq"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type reportRequest struct {
	Events []reportedEvent `json:"events"`
}

// runState is what the Runner needs back after a report: where the Run
// stands and the seq to send next.
func runState(r domain.Run) contractshttp.Json {
	return contractshttp.Json{"run": contractshttp.Json{
		"id": r.ID, "status": string(r.Status), "next_seq": r.NextSeq, "session_id": r.SessionID,
		"lease_expires_at": utc(r.LeaseExpiresAt),
	}}
}

// AppendRunEvents keeps the Run events this Desktop reports, in order by
// seq; a seq already kept is ignored, a gap is 422 with expected_seq.
func (c *DesktopController) AppendRunEvents(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req reportRequest
	if err := json.NewDecoder(ctx.Request().Origin().Body).Decode(&req); err != nil {
		return respond.BadBody(ctx)
	}
	if len(req.Events) > maxReportEvents {
		return respond.Error(ctx, contractshttp.StatusRequestEntityTooLarge, "at most 500 events in one report")
	}
	events := make([]domain.RunEvent, len(req.Events))
	for i, e := range req.Events {
		if len(e.Payload) > maxEventPayload {
			return respond.Error(ctx, contractshttp.StatusRequestEntityTooLarge, "an event payload is at most 256 KiB")
		}
		if len(e.Payload) > 0 && !json.Valid(e.Payload) {
			return respond.Invalid(ctx, "events", "payload must be JSON")
		}
		events[i] = domain.RunEvent{Seq: e.Seq, Kind: domain.EventKind(e.Kind), Payload: e.Payload}
	}
	r, err := c.service.AppendRunEvents(ctx.Context(), c.desktop(ctx), id, events)
	if err != nil {
		return desktopFail(ctx, err)
	}
	return ctx.Response().Success().Json(runState(r))
}

// KeepRunLease renews the Lease of the Run this Desktop runs.
func (c *DesktopController) KeepRunLease(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	r, err := c.service.KeepRunLease(ctx.Context(), c.desktop(ctx), id)
	if err != nil {
		return desktopFail(ctx, err)
	}
	return ctx.Response().Success().Json(runState(r))
}

type finishRequest struct {
	Status   string    `json:"status"`
	ExitCode *int      `json:"exit_code"`
	Error    string    `json:"error"`
	Usage    usageJSON `json:"usage"`
}

// FinishRun ends the Run this Desktop ran, with its Run usage; one
// cancelled meanwhile is answered as it is.
func (c *DesktopController) FinishRun(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req finishRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	u := req.Usage
	r, err := c.service.FinishRun(ctx.Context(), c.desktop(ctx), id, app.Finish{
		Status: req.Status, ExitCode: req.ExitCode, Error: req.Error,
		Usage: domain.Usage{
			InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, OutputTokens: u.OutputTokens,
			Turns: u.Turns, CostEquivalentUSD: u.CostEquivalentUSD, DurationMS: u.DurationMS,
		},
	})
	if err != nil {
		return desktopFail(ctx, err)
	}
	return ctx.Response().Success().Json(runState(r))
}
