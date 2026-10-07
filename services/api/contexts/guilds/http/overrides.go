package http

import (
	"context"
	"errors"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// ProjectOf finds the Project and Guild of the thing of one kind with this
// id, as the context that owns it answers; false when there is none.
type ProjectOf func(ctx context.Context, id uint64) (projectID, guildID uint64, found bool, err error)

// InProject answers 404 when the route's {id} names something outside the
// Current guild, or in a Project the request may not view; else the
// request's Permissions are resolved in that Project, for Can and Allows
// after it. A malformed {id} is left to the handler. It runs after Auth.
type InProject struct {
	Service *app.Service
	// Name tells the middlewares apart, e.g. "application".
	Name      string
	ProjectOf ProjectOf
}

func (m InProject) Signature() string { return "guilds.in-project." + m.Name }

func (m InProject) Handle(ctx contractshttp.Context) {
	id, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	if err != nil {
		ctx.Request().Next()
		return
	}
	pl, found, err := m.resolve(ctx.Context(), placeOf(ctx), id)
	if err != nil {
		_ = respond.ServerError(ctx, err).Abort()
		return
	}
	if !found {
		_ = respond.Error(ctx, contractshttp.StatusNotFound, "not found").Abort()
		return
	}
	ctx.WithValue(placeKey{}, pl)
	ctx.Request().Next()
}

// resolve is pl in the Project of the thing with this id; false when it is
// outside pl's Guild or in a Project pl may not view.
func (m InProject) resolve(ctx context.Context, pl place, id uint64) (place, bool, error) {
	projectID, guildID, found, err := m.ProjectOf(ctx, id)
	if err != nil || !found || guildID != pl.guild.ID {
		return place{}, false, err
	}
	held, err := m.Service.InProject(ctx, pl.at, projectID)
	if err != nil {
		return place{}, false, err
	}
	perms := effective(pl.principal, held)
	if !perms.Has(domain.PermissionViewResources) {
		return place{}, false, nil
	}
	pl.held, pl.permissions, pl.project = held, perms, projectID
	return pl, true, nil
}

// VisibleProjects keeps the Projects of the Current guild among ids that
// the request may view, in their order.
func VisibleProjects(ctx contractshttp.Context, s *app.Service, ids []uint64) ([]uint64, error) {
	pl := placeOf(ctx)
	if !pl.permissions.Has(domain.PermissionViewResources) {
		return []uint64{}, nil
	}
	return s.VisibleProjects(ctx.Context(), pl.at, ids)
}

type overrideJSON struct {
	RoleID   *uint64  `json:"role_id"`
	MemberID *uint64  `json:"member_id"`
	Allow    []string `json:"allow"`
	Deny     []string `json:"deny"`
}

func overrideToJSON(o domain.Override) overrideJSON {
	out := overrideJSON{Allow: o.Allow.Keys(), Deny: o.Deny.Keys()}
	if o.RoleID != 0 {
		out.RoleID = &o.RoleID
	}
	if o.MemberID != 0 {
		out.MemberID = &o.MemberID
	}
	return out
}

// ProjectPermissions lists the Permission overrides of the request's
// Project (InProject for "project").
func (c *Controller) ProjectPermissions(ctx contractshttp.Context) contractshttp.Response {
	pl := placeOf(ctx)
	os, err := c.service.Overrides(ctx.Context(), pl.guild.ID, pl.project)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]overrideJSON, len(os))
	for i, o := range os {
		out[i] = overrideToJSON(o)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"overrides": out})
}

type overrideRequest struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// OverrideRole sets the Project's override for the Role {role_id}.
func (c *Controller) OverrideRole(ctx contractshttp.Context) contractshttp.Response {
	return c.override(ctx, "role_id", true, true)
}

// OverrideMember sets the Project's override for the Member {member_id}.
func (c *Controller) OverrideMember(ctx contractshttp.Context) contractshttp.Response {
	return c.override(ctx, "member_id", false, true)
}

// ForgetRoleOverride deletes the Project's override for the Role.
func (c *Controller) ForgetRoleOverride(ctx contractshttp.Context) contractshttp.Response {
	return c.override(ctx, "role_id", true, false)
}

// ForgetMemberOverride deletes the Project's override for the Member.
func (c *Controller) ForgetMemberOverride(ctx contractshttp.Context) contractshttp.Response {
	return c.override(ctx, "member_id", false, false)
}

func (c *Controller) override(ctx contractshttp.Context, param string, role, set bool) contractshttp.Response {
	who, err := strconv.ParseUint(ctx.Request().Route(param), 10, 64)
	if err != nil {
		if role {
			return respond.Error(ctx, contractshttp.StatusNotFound, "role not found")
		}
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	var allow, deny domain.Permissions
	if set {
		var req overrideRequest
		if err := ctx.Request().Bind(&req); err != nil {
			return respond.BadBody(ctx)
		}
		if allow, err = domain.ParsePermissions(req.Allow); err != nil {
			return respond.Invalid(ctx, "allow", err.Error())
		}
		if deny, err = domain.ParsePermissions(req.Deny); err != nil {
			return respond.Invalid(ctx, "deny", err.Error())
		}
	}
	roleID, memberID := who, uint64(0)
	if !role {
		roleID, memberID = 0, who
	}
	pl := placeOf(ctx)
	o, err := c.service.SetOverride(ctx.Context(), pl.guild.ID, MemberID(ctx), pl.permissions, pl.project, roleID, memberID, allow, deny)
	if errors.Is(err, domain.ErrNotOverridable) {
		return respond.Invalid(ctx, "allow", err.Error())
	}
	if err != nil {
		return roleFailure(ctx, err)
	}
	if !set {
		return ctx.Response().NoContent()
	}
	return ctx.Response().Success().Json(contractshttp.Json{"override": overrideToJSON(o)})
}
