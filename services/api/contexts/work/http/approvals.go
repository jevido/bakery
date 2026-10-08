package http

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
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

type approvalJSON struct {
	ID           uint64      `json:"id"`
	Type         string      `json:"type"`
	Status       string      `json:"status"`
	Payload      payloadJSON `json:"payload"`
	Requester    *Member     `json:"requester"`
	DecidedBy    *Member     `json:"decided_by"`
	DecisionNote *string     `json:"decision_note"`
	DecidedAt    *time.Time  `json:"decided_at"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// approvalsJSON shows Approvals with their Requesters' and deciders'
// names, asked for in one go.
func (c *Controller) approvalsJSON(ctx context.Context, as []domain.Approval) ([]approvalJSON, error) {
	var ids []uint64
	for _, a := range as {
		for _, id := range []uint64{a.RequesterID, a.DeciderID} {
			if id != 0 {
				ids = append(ids, id)
			}
		}
	}
	names := map[uint64]Member{}
	if len(ids) > 0 {
		ms, err := c.members(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			names[m.ID] = m
		}
	}
	member := func(id uint64) *Member {
		if m, ok := names[id]; ok {
			return &m
		}
		return nil
	}
	out := make([]approvalJSON, len(as))
	for i, a := range as {
		p := a.Payload
		out[i] = approvalJSON{
			ID: a.ID, Type: string(a.Type), Status: string(a.Status),
			Payload: payloadJSON{
				Title: p.Title, Summary: p.Summary, RecommendedAction: p.RecommendedAction,
				NextActionOnApproval: p.NextActionOnApproval, Risks: p.Risks,
			},
			Requester: member(a.RequesterID), DecidedBy: member(a.DeciderID),
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
	out, err := c.approvalsJSON(ctx.Context(), as)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"approvals": out})
}

func (c *Controller) oneApproval(ctx contractshttp.Context, status int, a domain.Approval, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.approvalsJSON(ctx.Context(), []domain.Approval{a})
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
	p, err := boardPayload(req.Payload)
	if err != nil {
		return fail(ctx, err)
	}
	a, err := c.service.RequestApproval(ctx.Context(), c.guild(ctx), c.Member(ctx), req.Type, p, req.IssueIDs, c.visible(ctx))
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
