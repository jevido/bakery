// Package http is the agents JSON API: the Current guild's Agents, hiring
// one, and its Org chart.
package http

import (
	"context"
	"errors"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// Named is someone or something shown by name: a Hirer or a Manager.
type Named struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type Controller struct {
	service *app.Service
	// Guild is the Current guild of a request, Member the Member it comes
	// from and Permissions what it may do there (guilds).
	Guild       func(ctx contractshttp.Context) uint64
	Member      func(ctx contractshttp.Context) uint64
	Permissions func(ctx contractshttp.Context) []string
	// Members names the Members with these ids (identity.Members); a
	// removed Member is left out.
	Members func(ctx context.Context, ids []uint64) ([]Named, error)
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

type roleJSON struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

type agentJSON struct {
	ID           uint64     `json:"id"`
	Name         string     `json:"name"`
	Job          string     `json:"job"`
	JobLabel     string     `json:"job_label"`
	Title        string     `json:"title"`
	Icon         string     `json:"icon"`
	Capabilities string     `json:"capabilities"`
	Status       string     `json:"status"`
	ReportsTo    *Named     `json:"reports_to"`
	Hirer        *Named     `json:"hirer"`
	Roles        []roleJSON `json:"roles"`
	ApprovalID   *uint64    `json:"approval_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	PausedAt     *time.Time `json:"paused_at"`
	TerminatedAt *time.Time `json:"terminated_at"`
}

// agentsJSON shows Agents with their Managers, Hirers and Roles. all is
// the Guild's Agents, to name Managers from.
func (c *Controller) agentsJSON(ctx context.Context, as []domain.Agent, all []domain.Agent) ([]agentJSON, error) {
	names := map[uint64]string{}
	for _, a := range all {
		names[a.ID] = a.Name
	}
	var ids []uint64
	for _, a := range as {
		ids = append(ids, a.HirerID)
	}
	hirers := map[uint64]Named{}
	if len(ids) > 0 {
		ms, err := c.Members(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			hirers[m.ID] = m
		}
	}
	out := make([]agentJSON, len(as))
	for i, a := range as {
		roles, err := c.service.Roles(ctx, a)
		if err != nil {
			return nil, err
		}
		j := agentJSON{
			ID: a.ID, Name: a.Name, Job: string(a.Job), JobLabel: domain.JobLabel(a.Job), Title: a.Title, Icon: string(a.Icon),
			Capabilities: a.Capabilities, Status: string(a.Status), Roles: make([]roleJSON, len(roles)),
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, PausedAt: a.PausedAt, TerminatedAt: a.TerminatedAt,
		}
		for k, r := range roles {
			j.Roles[k] = roleJSON{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position}
		}
		if name, ok := names[a.ManagerID]; ok && a.ManagerID != 0 {
			j.ReportsTo = &Named{ID: a.ManagerID, Name: name}
		}
		if h, ok := hirers[a.HirerID]; ok {
			j.Hirer = &h
		}
		if a.HireApprovalID != 0 {
			id := a.HireApprovalID
			j.ApprovalID = &id
		}
		out[i] = j
	}
	return out, nil
}

type nodeJSON struct {
	ID       uint64     `json:"id"`
	Name     string     `json:"name"`
	Job      string     `json:"job"`
	JobLabel string     `json:"job_label"`
	Title    string     `json:"title"`
	Icon     string     `json:"icon"`
	Status   string     `json:"status"`
	Reports  []nodeJSON `json:"reports"`
}

func nodesJSON(ns []domain.Node) []nodeJSON {
	out := make([]nodeJSON, len(ns))
	for i, n := range ns {
		a := n.Agent
		out[i] = nodeJSON{
			ID: a.ID, Name: a.Name, Job: string(a.Job), JobLabel: domain.JobLabel(a.Job), Title: a.Title,
			Icon: string(a.Icon), Status: string(a.Status), Reports: nodesJSON(n.Reports),
		}
	}
	return out
}

type hireRequest struct {
	Name         string   `json:"name"`
	Job          string   `json:"job"`
	Title        string   `json:"title"`
	Icon         string   `json:"icon"`
	Capabilities string   `json:"capabilities"`
	ReportsTo    *uint64  `json:"reports_to"`
	RoleIDs      []uint64 `json:"role_ids"`
}

func routeID(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func notFound(ctx contractshttp.Context) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
}

func fail(ctx contractshttp.Context, err error) contractshttp.Response {
	var fe *domain.FieldError
	var se *domain.StatusError
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.As(err, &se):
		return respond.Error(ctx, contractshttp.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, app.ErrNotFound):
		return notFound(ctx)
	}
	return respond.ServerError(ctx, err)
}

// ListAgents answers the Current guild's Agents by name, in the status
// filter (all, active, paused, pending or terminated).
func (c *Controller) ListAgents(ctx contractshttp.Context) contractshttp.Response {
	guild := c.Guild(ctx)
	as, err := c.service.Agents(ctx.Context(), guild, ctx.Request().Query("status", ""))
	if err != nil {
		return fail(ctx, err)
	}
	all, err := c.service.Agents(ctx.Context(), guild, "")
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.agentsJSON(ctx.Context(), as, all)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"agents": out})
}

func (c *Controller) oneAgent(ctx contractshttp.Context, a domain.Agent) (agentJSON, error) {
	var all []domain.Agent
	if a.ManagerID != 0 {
		m, err := c.service.Agent(ctx.Context(), a.GuildID, a.ManagerID)
		if err != nil && !errors.Is(err, app.ErrNotFound) {
			return agentJSON{}, err
		}
		all = append(all, m)
	}
	out, err := c.agentsJSON(ctx.Context(), []domain.Agent{a}, all)
	if err != nil {
		return agentJSON{}, err
	}
	return out[0], nil
}

// HireAgent hires an Agent with the asking Member as its Hirer, and
// answers it with its hire_agent Approval's id.
func (c *Controller) HireAgent(ctx contractshttp.Context) contractshttp.Response {
	var req hireRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	var manager uint64
	if req.ReportsTo != nil {
		manager = *req.ReportsTo
	}
	a, approvalID, err := c.service.Hire(ctx.Context(), c.Guild(ctx), c.Member(ctx), c.Permissions(ctx), app.HireInput{
		Profile:   domain.Profile{Name: req.Name, Job: req.Job, Title: req.Title, Icon: req.Icon, Capabilities: req.Capabilities},
		ManagerID: manager, RoleIDs: req.RoleIDs,
	})
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.oneAgent(ctx, a)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"agent": out, "approval_id": approvalID})
}

func (c *Controller) ShowAgent(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	a, err := c.service.Agent(ctx.Context(), c.Guild(ctx), id)
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.oneAgent(ctx, a)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"agent": out})
}

// ShowOrg answers the roots of the Current guild's Org chart.
func (c *Controller) ShowOrg(ctx contractshttp.Context) contractshttp.Response {
	org, err := c.service.Org(ctx.Context(), c.Guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"org": nodesJSON(org)})
}
