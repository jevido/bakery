// Package http is the work JSON API: Goals, Issues and their Comments.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Member is a Member as a Goal or an Issue shows them.
type Member struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type Controller struct {
	service *app.Service
	// guild is the Current guild of a request.
	guild func(ctx contractshttp.Context) uint64
	// members names the Members with these ids (identity.Members); a
	// removed Member is left out.
	members func(ctx context.Context, ids []uint64) ([]Member, error)
	// Visible keeps the Projects among ids the request may view
	// (guilds.VisibleProjects); Member is the Member it comes from
	// (guilds.MemberID). Both must be set before Issues are served.
	Visible func(ctx contractshttp.Context, ids []uint64) ([]uint64, error)
	Member  func(ctx contractshttp.Context) uint64
	// Agent is the Agent a Run key acts as (guilds.AgentID), 0 for a person;
	// nil counts every request as a person's.
	Agent func(ctx contractshttp.Context) uint64
	// Run is the Run a Run key acts in (guilds.RunID), 0 for a person.
	Run func(ctx contractshttp.Context) uint64
	// AgentNames names the Guild's Agents with these ids that still exist;
	// nil (or a nil answer) counts every Agent as existing.
	AgentNames func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error)
	// SkillNames names the Guild's Skills with these ids that still exist;
	// nil (or a nil answer) counts every Skill as existing.
	SkillNames func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error)
	// DashboardURL is where the dashboard is reached, for links to an
	// Issue page.
	DashboardURL func() string
}

func NewController(service *app.Service, guild func(ctx contractshttp.Context) uint64, members func(ctx context.Context, ids []uint64) ([]Member, error)) *Controller {
	return &Controller{service: service, guild: guild, members: members}
}

// optional is a JSON field that may be absent, null or a value: a patch
// tells "leave it" (absent) from "clear it" (null).
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	return json.Unmarshal(b, &o.Value)
}

// idOf reads an optional id; null and 0 are none.
func idOf(o optional[uint64]) *uint64 {
	if !o.Set {
		return nil
	}
	v := value(o.Value)
	return &v
}

func (o optional[T]) ptr() *T {
	if !o.Set || o.Value == nil {
		return nil
	}
	return o.Value
}

type goalJSON struct {
	ID          uint64    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Level       string    `json:"level"`
	Status      string    `json:"status"`
	ParentID    *uint64   `json:"parent_id"`
	Owner       *Member   `json:"owner"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// goalsJSON shows Goals with their owners' names, asked for in one go.
func (c *Controller) goalsJSON(ctx context.Context, gs []domain.Goal) ([]goalJSON, error) {
	var ids []uint64
	for _, g := range gs {
		if g.OwnerID != 0 {
			ids = append(ids, g.OwnerID)
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
	out := make([]goalJSON, len(gs))
	for i, g := range gs {
		out[i] = goalJSON{
			ID: g.ID, Title: g.Title, Description: g.Description, Level: string(g.Level), Status: string(g.Status),
			CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
		}
		if g.ParentID != 0 {
			p := g.ParentID
			out[i].ParentID = &p
		}
		if m, ok := names[g.OwnerID]; ok {
			out[i].Owner = &m
		}
	}
	return out, nil
}

type goalRequest struct {
	Title       optional[string] `json:"title"`
	Description optional[string] `json:"description"`
	Level       optional[string] `json:"level"`
	Status      optional[string] `json:"status"`
	ParentID    optional[uint64] `json:"parent_id"`
	OwnerID     optional[uint64] `json:"owner_id"`
}

func value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func (r goalRequest) input() app.GoalInput {
	return app.GoalInput{
		Title: value(r.Title.ptr()), Description: value(r.Description.ptr()), Level: value(r.Level.ptr()), Status: value(r.Status.ptr()),
		ParentID: value(idOf(r.ParentID)), OwnerID: value(idOf(r.OwnerID)),
	}
}

func (r goalRequest) patch() app.GoalPatch {
	p := app.GoalPatch{Title: r.Title.ptr(), Description: r.Description.ptr(), Level: r.Level.ptr(), Status: r.Status.ptr(), ParentID: idOf(r.ParentID), OwnerID: idOf(r.OwnerID)}
	if r.Description.Set && p.Description == nil {
		// A null description is an empty one.
		p.Description = new(string)
	}
	return p
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
	var stale *app.StaleRevisionError
	var staleRoutine *app.StaleRoutineRevisionError
	var newestRoutine *app.RestoreNewestRoutineError
	var refused *domain.ApprovalRefusedError
	var held *domain.HeldError
	var prRefused *app.PullRequestRefusedError
	var status *domain.StatusError
	switch {
	case errors.As(err, &held):
		return ctx.Response().Json(contractshttp.StatusConflict, contractshttp.Json{"message": err.Error(), "run_id": held.RunID})
	case errors.As(err, &status):
		return ctx.Response().Json(contractshttp.StatusConflict, contractshttp.Json{"message": err.Error(), "status": string(status.Status)})
	case errors.Is(err, domain.ErrNotHolder), errors.Is(err, domain.ErrNotAssignee), errors.Is(err, app.ErrBusy), errors.Is(err, domain.ErrArchivedRoutineTriggers),
		errors.Is(err, domain.ErrRoutineArchivedRun), errors.Is(err, domain.ErrRestoreArchivedRoutine), errors.Is(err, domain.ErrRoutinePaused), errors.Is(err, domain.ErrTriggerDisabled):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case errors.Is(err, app.ErrAgentsOnly), errors.Is(err, app.ErrNotOwnRoutine), errors.Is(err, domain.ErrNotRoutinesTrigger):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.As(err, &refused), errors.As(err, &prRefused):
		return respond.Error(ctx, contractshttp.StatusUnprocessableEntity, err.Error())
	case errors.As(err, &stale):
		// The newest Revision goes along, so the dashboard can offer to
		// reload instead of overwriting it.
		return ctx.Response().Json(contractshttp.StatusConflict, contractshttp.Json{
			"message":                 err.Error(),
			"current_revision_id":     stale.Current.LatestRevisionID,
			"current_revision_number": stale.Current.Latest,
		})
	case errors.As(err, &staleRoutine):
		return ctx.Response().Json(contractshttp.StatusConflict, contractshttp.Json{
			"message":                 err.Error(),
			"current_revision_id":     staleRoutine.Current.LatestRevisionID,
			"current_revision_number": staleRoutine.Current.LatestRevisionNumber,
		})
	case errors.As(err, &newestRoutine):
		return ctx.Response().Json(contractshttp.StatusConflict, contractshttp.Json{
			"message":                 err.Error(),
			"current_revision_id":     newestRoutine.Current.LatestRevisionID,
			"current_revision_number": newestRoutine.Current.LatestRevisionNumber,
		})
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.Is(err, app.ErrNotFound):
		return notFound(ctx)
	case errors.Is(err, domain.ErrNotAuthor), errors.Is(err, domain.ErrNotRequester), errors.Is(err, domain.ErrNotConversationOwner),
		errors.Is(err, app.ErrPeopleOnly):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrCommentDeleted), errors.Is(err, domain.ErrRestoreNewest),
		errors.Is(err, app.ErrDocumentExists), errors.Is(err, app.ErrNoDocumentYet):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	}
	return respond.ServerError(ctx, err)
}

func (c *Controller) oneGoal(ctx contractshttp.Context, status int, g domain.Goal, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.goalsJSON(ctx.Context(), []domain.Goal{g})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"goal": out[0]})
}

// ListGoals answers the Current guild's Goals as a flat list, oldest
// first; the dashboard builds the tree from parent_id.
func (c *Controller) ListGoals(ctx contractshttp.Context) contractshttp.Response {
	gs, err := c.service.Goals(ctx.Context(), c.guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.goalsJSON(ctx.Context(), gs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"goals": out})
}

func (c *Controller) CreateGoal(ctx contractshttp.Context) contractshttp.Response {
	var req goalRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	g, err := c.service.CreateGoal(ctx.Context(), c.guild(ctx), c.Member(ctx), req.input())
	return c.oneGoal(ctx, contractshttp.StatusCreated, g, err)
}

func (c *Controller) ShowGoal(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	g, err := c.service.Goal(ctx.Context(), id)
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.goalsJSON(ctx.Context(), []domain.Goal{g})
	if err != nil {
		return fail(ctx, err)
	}
	issues, counts, err := c.goalIssues(ctx, g)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"goal": struct {
		goalJSON
		Issues      []issueJSON    `json:"issues"`
		IssueCounts map[string]int `json:"issue_counts"`
	}{out[0], issues, counts}})
}

func (c *Controller) UpdateGoal(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req goalRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	g, err := c.service.ChangeGoal(ctx.Context(), c.Member(ctx), id, req.patch())
	return c.oneGoal(ctx, contractshttp.StatusOK, g, err)
}

func (c *Controller) DeleteGoal(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteGoal(ctx.Context(), c.Member(ctx), id); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
