package http

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// payloadJSON is a request_board_approval payload on the wire, in
// snake_case.
type payloadJSON struct {
	Title                string   `json:"title"`
	Summary              string   `json:"summary"`
	RecommendedAction    string   `json:"recommended_action"`
	NextActionOnApproval string   `json:"next_action_on_approval"`
	Risks                []string `json:"risks"`
}

// agentRefJSON is an Agent as a hire_agent payload names it.
type agentRefJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// hirePayloadJSON is a hire_agent payload on the wire, in snake_case.
type hirePayloadJSON struct {
	AgentID      uint64        `json:"agent_id"`
	Name         string        `json:"name"`
	Job          string        `json:"job"`
	Title        string        `json:"title"`
	Icon         string        `json:"icon"`
	Capabilities string        `json:"capabilities"`
	ReportsTo    *agentRefJSON `json:"reports_to"`
	Roles        []string      `json:"roles"`
}

// budgetPayloadJSON is a budget_override_required payload on the wire.
type budgetPayloadJSON struct {
	BudgetID    uint64     `json:"budget_id"`
	ScopeType   string     `json:"scope_type"`
	ScopeID     uint64     `json:"scope_id"`
	ScopeName   string     `json:"scope_name"`
	Metric      string     `json:"metric"`
	Window      string     `json:"window"`
	Threshold   string     `json:"threshold"`
	Amount      int64      `json:"amount"`
	Observed    int64      `json:"observed"`
	WarnPercent int        `json:"warn_percent"`
	WindowStart *time.Time `json:"window_start"`
	WindowEnd   *time.Time `json:"window_end"`
	Guidance    string     `json:"guidance"`
}

// payloadOut is the payload as its type has it on the wire.
func payloadOut(p domain.ApprovalPayload) any {
	switch p := p.(type) {
	case domain.BudgetOverridePayload:
		return budgetPayloadJSON(p)
	case domain.HireAgentPayload:
		out := hirePayloadJSON{AgentID: p.AgentID, Name: p.Name, Job: p.Job, Title: p.Title, Icon: p.Icon, Capabilities: p.Capabilities, Roles: p.Roles}
		if p.ManagerID != 0 {
			out.ReportsTo = &agentRefJSON{ID: p.ManagerID, Name: p.ManagerName}
		}
		return out
	case domain.BoardApprovalPayload:
		return payloadJSON{
			Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
			NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
		}
	}
	return nil
}

type approvalJSON struct {
	ID             uint64     `json:"id"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Payload        any        `json:"payload"`
	Requester      *Member    `json:"requester"`
	RequesterAgent *Agent     `json:"requester_agent"`
	DecidedBy      *Member    `json:"decided_by"`
	DecisionNote   *string    `json:"decision_note"`
	DecidedAt      *time.Time `json:"decided_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// approvalsJSON shows Approvals with their Requesters' and deciders'
// names, asked for in one go.
func (c *Controller) approvalsJSON(ctx contractshttp.Context, as []domain.Approval) ([]approvalJSON, error) {
	var actors []domain.Actor
	for _, a := range as {
		actors = append(actors, a.Requester, domain.ByMember(a.DeciderID))
	}
	names, err := c.actorNames(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := make([]approvalJSON, len(as))
	for i, a := range as {
		out[i] = approvalJSON{
			ID: a.ID, Type: string(a.Type), Status: string(a.Status), Payload: payloadOut(a.Payload),
			Requester: names.member(a.Requester), RequesterAgent: names.agent(a.Requester), DecidedBy: names.member(domain.ByMember(a.DeciderID)),
			DecidedAt: a.DecidedAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		}
		if a.DecisionNote != "" {
			note := a.DecisionNote
			out[i].DecisionNote = &note
		}
	}
	return out, nil
}

func (c *Controller) approvalsResponse(ctx contractshttp.Context, as []domain.Approval, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.approvalsJSON(ctx, as)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"approvals": out})
}

func (c *Controller) oneApproval(ctx contractshttp.Context, status int, a domain.Approval, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.approvalsJSON(ctx, []domain.Approval{a})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"approval": out[0]})
}

// boardPayload reads a request_board_approval payload; a field it does not
// know breaks the rule that it has nothing else.
func boardPayload(raw json.RawMessage) (domain.BoardApprovalPayload, error) {
	var p payloadJSON
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if len(raw) == 0 || dec.Decode(&p) != nil {
		return domain.BoardApprovalPayload{}, &domain.FieldError{Field: "payload", Message: "payload must be an object of title, summary, recommended_action, next_action_on_approval and risks"}
	}
	return domain.BoardApprovalPayload{
		Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
		NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
	}, nil
}

// ListApprovals answers the Current guild's Approvals, newest first:
// status is a comma list of Approval statuses or actionable.
func (c *Controller) ListApprovals(ctx contractshttp.Context) contractshttp.Response {
	as, err := c.service.Approvals(ctx.Context(), c.guild(ctx), ctx.Request().Query("status"))
	return c.approvalsResponse(ctx, as, err)
}

// RequestApproval asks the Board to decide, as the asking Member.
func (c *Controller) RequestApproval(ctx contractshttp.Context) contractshttp.Response {
	var req struct {
		Type     string          `json:"type"`
		Payload  json.RawMessage `json:"payload"`
		IssueIDs []uint64        `json:"issue_ids"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	// The type goes first, so a hire_agent is refused for what it is,
	// not for its payload.
	if _, err := app.RequestableType(req.Type); err != nil {
		return fail(ctx, err)
	}
	p, err := boardPayload(req.Payload)
	if err != nil {
		return fail(ctx, err)
	}
	a, err := c.service.RequestApproval(ctx.Context(), c.guild(ctx), c.actor(ctx), req.Type, p, req.IssueIDs, c.visible(ctx))
	return c.oneApproval(ctx, contractshttp.StatusCreated, a, err)
}

func (c *Controller) ShowApproval(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	a, err := c.service.Approval(ctx.Context(), c.guild(ctx), id)
	return c.oneApproval(ctx, contractshttp.StatusOK, a, err)
}

// ListApprovalIssues answers the Approval's Linked issues the request may
// view, as in Issue lists.
func (c *Controller) ListApprovalIssues(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	is, err := c.service.ApprovalIssues(ctx.Context(), c.guild(ctx), id, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.issuesJSON(ctx, is, false)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"issues": out})
}

// ListIssueApprovals answers the Approvals linked to the Issue, newest
// first.
func (c *Controller) ListIssueApprovals(ctx contractshttp.Context) contractshttp.Response {
	as, err := c.service.IssueApprovals(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), c.visible(ctx))
	return c.approvalsResponse(ctx, as, err)
}

type decisionRequest struct {
	DecisionNote string `json:"decision_note"`
}

// decide runs a Decision on the route's Approval with the request's
// Decision note.
func (c *Controller) decide(ctx contractshttp.Context, do func(ctx context.Context, guildID, memberID, id uint64, note string) (domain.Approval, error)) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req decisionRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	a, err := do(ctx.Context(), c.guild(ctx), c.Member(ctx), id, req.DecisionNote)
	return c.oneApproval(ctx, contractshttp.StatusOK, a, err)
}

func (c *Controller) ApproveApproval(ctx contractshttp.Context) contractshttp.Response {
	return c.decide(ctx, c.service.ApproveApproval)
}

func (c *Controller) RejectApproval(ctx contractshttp.Context) contractshttp.Response {
	return c.decide(ctx, c.service.RejectApproval)
}

func (c *Controller) RequestApprovalRevision(ctx contractshttp.Context) contractshttp.Response {
	return c.decide(ctx, c.service.RequestApprovalRevision)
}

// ResubmitApproval makes one's own Approval pending again, with a new
// payload when the body has one.
func (c *Controller) ResubmitApproval(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	var p domain.ApprovalPayload
	if len(req.Payload) > 0 && string(req.Payload) != "null" {
		bp, err := boardPayload(req.Payload)
		if err != nil {
			return fail(ctx, err)
		}
		p = bp
	}
	a, err := c.service.ResubmitApproval(ctx.Context(), c.guild(ctx), c.Member(ctx), id, p)
	return c.oneApproval(ctx, contractshttp.StatusOK, a, err)
}

// approvalCommentJSON is a comment in an Approval's thread.
type approvalCommentJSON struct {
	ID          uint64    `json:"id"`
	Body        string    `json:"body"`
	Author      *Member   `json:"author"`
	AuthorAgent *Agent    `json:"author_agent"`
	CreatedAt   time.Time `json:"created_at"`
}

// approvalCommentsJSON shows Approval comments with their authors' names,
// asked for in one go.
func (c *Controller) approvalCommentsJSON(ctx contractshttp.Context, cs []domain.ApprovalComment) ([]approvalCommentJSON, error) {
	authors := make([]domain.Actor, len(cs))
	for n, cm := range cs {
		authors[n] = cm.Author
	}
	names, err := c.actorNames(ctx, authors)
	if err != nil {
		return nil, err
	}
	out := make([]approvalCommentJSON, len(cs))
	for n, cm := range cs {
		out[n] = approvalCommentJSON{ID: cm.ID, Body: cm.Body, CreatedAt: cm.CreatedAt, Author: names.member(cm.Author), AuthorAgent: names.agent(cm.Author)}
	}
	return out, nil
}

// ListApprovalComments answers the Approval's thread, oldest first.
func (c *Controller) ListApprovalComments(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	cs, err := c.service.ApprovalComments(ctx.Context(), c.guild(ctx), id)
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.approvalCommentsJSON(ctx, cs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"comments": out})
}

func (c *Controller) AddApprovalComment(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req commentRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	cm, err := c.service.AddApprovalComment(ctx.Context(), c.guild(ctx), c.actor(ctx), id, req.Body)
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.approvalCommentsJSON(ctx, []domain.ApprovalComment{cm})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"comment": out[0]})
}
