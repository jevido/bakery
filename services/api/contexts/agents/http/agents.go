// Package http is the agents JSON API: the Current guild's Agents, hiring
// and managing one, its Org chart, and the Agents' Runs.
package http

import (
	"context"
	"encoding/json"
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
	// Agent is the Agent a Run key's request comes from, 0 for a
	// person's (guilds.AgentID); nil reads as 0.
	Agent func(ctx contractshttp.Context) uint64
	// Run is the Run whose key the request carries, 0 for a person's
	// (guilds.RunID).
	Run func(ctx contractshttp.Context) uint64
	// InstanceAdmin reports whether the request comes from the Instance
	// admin.
	InstanceAdmin func(ctx contractshttp.Context) bool
	// Visible keeps the Projects among ids that the request may view
	// (guilds.VisibleProjects).
	Visible func(ctx contractshttp.Context, ids []uint64) ([]uint64, error)
	// Desktops names the Desktops among ids (identity.DesktopNames).
	Desktops func(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// Shutdown closes when the API stops, ending open streams.
	Shutdown <-chan struct{}
}

func (c *Controller) actor(ctx contractshttp.Context) app.Actor {
	return app.Actor{ID: c.Member(ctx), Permissions: c.Permissions(ctx), InstanceAdmin: c.InstanceAdmin(ctx)}
}

// agent is the Agent asking through its Run key, 0 for a person. An Agent
// reads only its own Runs.
func (c *Controller) agent(ctx contractshttp.Context) uint64 {
	if c.Agent == nil {
		return 0
	}
	return c.Agent(ctx)
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
	ID           uint64        `json:"id"`
	Name         string        `json:"name"`
	Job          string        `json:"job"`
	JobLabel     string        `json:"job_label"`
	Title        string        `json:"title"`
	Icon         string        `json:"icon"`
	Capabilities string        `json:"capabilities"`
	Status       string        `json:"status"`
	ReportsTo    *Named        `json:"reports_to"`
	Hirer        *Named        `json:"hirer"`
	Roles        []roleJSON    `json:"roles"`
	ApprovalID   *uint64       `json:"approval_id"`
	CurrentRunID *uint64       `json:"current_run_id"`
	Heartbeat    heartbeatJSON `json:"heartbeat"`
	CanManage    bool          `json:"can_manage"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	PausedAt     *time.Time    `json:"paused_at"`
	PauseReason  *string       `json:"pause_reason"`
	TerminatedAt *time.Time    `json:"terminated_at"`
}

// heartbeatJSON is an Agent's Heartbeat policy.
type heartbeatJSON struct {
	Enabled         bool       `json:"enabled"`
	IntervalSec     int        `json:"interval_sec"`
	WakeOnDemand    bool       `json:"wake_on_demand"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`
}

// agentsJSON shows Agents with their Managers, Hirers, Roles and whether
// the actor may manage them. all is the Guild's Agents, to name Managers
// from.
func (c *Controller) agentsJSON(ctx context.Context, actor app.Actor, as []domain.Agent, all []domain.Agent) ([]agentJSON, error) {
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
	current, err := c.currentRuns(ctx, as)
	if err != nil {
		return nil, err
	}
	// Whether the actor may manage an Agent depends only on its Hirer.
	manages := map[uint64]bool{}
	out := make([]agentJSON, len(as))
	for i, a := range as {
		may, ok := manages[a.HirerID]
		if !ok {
			var err error
			if may, err = c.service.MayManage(ctx, actor, a); err != nil {
				return nil, err
			}
			manages[a.HirerID] = may
		}
		roles, err := c.service.Roles(ctx, a)
		if err != nil {
			return nil, err
		}
		j := agentJSON{
			ID: a.ID, Name: a.Name, Job: string(a.Job), JobLabel: domain.JobLabel(a.Job), Title: a.Title, Icon: string(a.Icon),
			Capabilities: a.Capabilities, Status: string(a.Status), Roles: make([]roleJSON, len(roles)), CanManage: may,
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, PausedAt: a.PausedAt, TerminatedAt: a.TerminatedAt,
			PauseReason: nullableText(string(a.PauseReason)),
			Heartbeat: heartbeatJSON{
				Enabled: a.Heartbeat.Enabled, IntervalSec: a.Heartbeat.IntervalSec, WakeOnDemand: a.Heartbeat.WakeOnDemand,
				LastHeartbeatAt: a.LastHeartbeatAt,
			},
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
		if id, ok := current[a.ID]; ok {
			j.CurrentRunID = &id
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

// nullableText answers "" as null.
func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
	case errors.Is(err, app.ErrBudgetStillExceeded):
		return respond.Error(ctx, contractshttp.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, app.ErrMayNotManage), errors.Is(err, app.ErrHirerNotMember):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
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
	// A terminated Agent's Manager may be terminated too, so look among
	// the listed ones as well.
	all = append(all, as...)
	out, err := c.agentsJSON(ctx.Context(), c.actor(ctx), as, all)
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
	out, err := c.agentsJSON(ctx.Context(), c.actor(ctx), []domain.Agent{a}, all)
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

// optional is a PATCH field: Set when the body names it, Value nil when
// it is null.
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	return json.Unmarshal(b, &o.Value)
}

// text is a PATCH text field; null is empty.
func (o optional[T]) text() *T {
	if !o.Set {
		return nil
	}
	if o.Value == nil {
		return new(T)
	}
	return o.Value
}

type editRequest struct {
	Name         optional[string] `json:"name"`
	Job          optional[string] `json:"job"`
	Title        optional[string] `json:"title"`
	Icon         optional[string] `json:"icon"`
	Capabilities optional[string] `json:"capabilities"`
	// ReportsTo null (or 0) reports to no one.
	ReportsTo optional[uint64] `json:"reports_to"`
	// Heartbeat changes the fields of the Heartbeat policy it names.
	Heartbeat *struct {
		Enabled      *bool `json:"enabled"`
		IntervalSec  *int  `json:"interval_sec"`
		WakeOnDemand *bool `json:"wake_on_demand"`
	} `json:"heartbeat"`
}

// answer answers the Agent after a change, or the change's error.
func (c *Controller) answer(ctx contractshttp.Context, a domain.Agent, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.oneAgent(ctx, a)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"agent": out})
}

// EditAgent changes any of the Agent's name, job, title, icon,
// capabilities, reports_to and heartbeat.
func (c *Controller) EditAgent(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req editRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	var heartbeat *domain.HeartbeatPatch
	if h := req.Heartbeat; h != nil {
		heartbeat = &domain.HeartbeatPatch{Enabled: h.Enabled, IntervalSec: h.IntervalSec, WakeOnDemand: h.WakeOnDemand}
	}
	a, err := c.service.Edit(ctx.Context(), c.Guild(ctx), c.actor(ctx), id, domain.Patch{
		Name: req.Name.text(), Job: req.Job.text(), Title: req.Title.text(), Icon: req.Icon.text(),
		Capabilities: req.Capabilities.text(), ManagerID: req.ReportsTo.text(), Heartbeat: heartbeat,
	})
	return c.answer(ctx, a, err)
}

type change func(ctx context.Context, guildID uint64, actor app.Actor, id uint64) (domain.Agent, error)

func (c *Controller) move(ctx contractshttp.Context, f change) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	a, err := f(ctx.Context(), c.Guild(ctx), c.actor(ctx), id)
	return c.answer(ctx, a, err)
}

func (c *Controller) PauseAgent(ctx contractshttp.Context) contractshttp.Response {
	return c.move(ctx, c.service.Pause)
}

func (c *Controller) ResumeAgent(ctx contractshttp.Context) contractshttp.Response {
	return c.move(ctx, c.service.Resume)
}

func (c *Controller) TerminateAgent(ctx contractshttp.Context) contractshttp.Response {
	return c.move(ctx, c.service.Terminate)
}

func (c *Controller) reRole(ctx contractshttp.Context, f func(ctx context.Context, guildID uint64, actor app.Actor, id, roleID uint64) (domain.Agent, error)) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	roleID, err := strconv.ParseUint(ctx.Request().Route("role_id"), 10, 64)
	if err != nil {
		return notFound(ctx)
	}
	a, err := f(ctx.Context(), c.Guild(ctx), c.actor(ctx), id, roleID)
	return c.answer(ctx, a, err)
}

// AddAgentRole gives the Agent the Role {role_id}.
func (c *Controller) AddAgentRole(ctx contractshttp.Context) contractshttp.Response {
	return c.reRole(ctx, c.service.AddRole)
}

// RemoveAgentRole takes the Role {role_id} from the Agent.
func (c *Controller) RemoveAgentRole(ctx contractshttp.Context) contractshttp.Response {
	return c.reRole(ctx, c.service.RemoveRole)
}
