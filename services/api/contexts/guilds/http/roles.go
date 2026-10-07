package http

import (
	"context"
	"errors"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

type roleJSON struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	Color       string   `json:"color"`
	Position    int      `json:"position"`
	Permissions []string `json:"permissions"`
	Base        bool     `json:"base"`
	// Members counts who holds it: every Member for the Base role.
	Members int `json:"members"`
}

// roleRef names a Role someone holds or an Invitation gives.
type roleRef struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// refs are the Roles among roles (by Position) whose ids are in ids, top
// first, without the Base role; never nil.
func refs(roles []domain.Role, ids []uint64) []roleRef {
	out := []roleRef{}
	for i := len(roles) - 1; i >= 0; i-- {
		r := roles[i]
		if !r.Base && containsID(ids, r.ID) {
			out = append(out, roleRef{ID: r.ID, Name: r.Name, Color: r.Color})
		}
	}
	return out
}

func containsID(ids []uint64, id uint64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func roleToJSON(r domain.Role, members int) roleJSON {
	return roleJSON{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position, Permissions: r.Permissions.Keys(), Base: r.Base, Members: members}
}

// rolesJSON lists the Current guild's Roles top first, the Base role last,
// with how many Members hold each.
func (c *Controller) rolesJSON(ctx contractshttp.Context) ([]roleJSON, error) {
	roles, err := c.service.RolesIn(ctx.Context(), Current(ctx))
	if err != nil {
		return nil, err
	}
	ms, err := c.service.MembershipsIn(ctx.Context(), Current(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]roleJSON, 0, len(roles))
	for i := len(roles) - 1; i >= 0; i-- {
		r, n := roles[i], 0
		for _, m := range ms {
			if r.Base || containsID(m.RoleIDs, r.ID) {
				n++
			}
		}
		out = append(out, roleToJSON(r, n))
	}
	return out, nil
}

// Roles lists the Current guild's Roles, for every Member.
func (c *Controller) Roles(ctx contractshttp.Context) contractshttp.Response {
	out, err := c.rolesJSON(ctx)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"roles": out})
}

type permissionJSON struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Overridable bool   `json:"overridable"`
}

// Permissions lists the fixed list of Permissions, so the dashboard never
// hard-codes it.
func (c *Controller) Permissions(ctx contractshttp.Context) contractshttp.Response {
	all := domain.All()
	out := make([]permissionJSON, len(all))
	for i, p := range all {
		out[i] = permissionJSON{Key: p.Key(), Name: p.Name(), Description: p.Description(), Overridable: p.Overridable()}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"permissions": out})
}

type roleRequest struct {
	Name        *string   `json:"name"`
	Color       *string   `json:"color"`
	Permissions *[]string `json:"permissions"`
}

func (r roleRequest) edit() (app.RoleEdit, error) {
	e := app.RoleEdit{Name: r.Name, Color: r.Color}
	if r.Permissions != nil {
		p, err := domain.ParsePermissions(*r.Permissions)
		if err != nil {
			return app.RoleEdit{}, err
		}
		e.Permissions = &p
	}
	return e, nil
}

// CreateRole makes a Role in the Current guild just above the Base role.
func (c *Controller) CreateRole(ctx contractshttp.Context) contractshttp.Response {
	var req roleRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	e, err := req.edit()
	if err != nil {
		return roleFailure(ctx, err)
	}
	var name, color string
	var perms domain.Permissions
	if e.Name != nil {
		name = *e.Name
	}
	if e.Color != nil {
		color = *e.Color
	}
	if e.Permissions != nil {
		perms = *e.Permissions
	}
	r, err := c.service.CreateRole(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), name, color, perms)
	if err != nil {
		return roleFailure(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"role": roleToJSON(r, 0)})
}

// EditRole changes a Role's name, color or Permissions; what the body
// leaves out stays.
func (c *Controller) EditRole(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "role not found")
	}
	var req roleRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	e, err := req.edit()
	if err != nil {
		return roleFailure(ctx, err)
	}
	r, err := c.service.EditRole(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), id, e)
	if err != nil {
		return roleFailure(ctx, err)
	}
	return c.oneRole(ctx, r.ID)
}

// oneRole answers the Role with this id as GET /api/roles lists it.
func (c *Controller) oneRole(ctx contractshttp.Context, id uint64) contractshttp.Response {
	all, err := c.rolesJSON(ctx)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	for _, r := range all {
		if r.ID == id {
			return ctx.Response().Success().Json(contractshttp.Json{"role": r})
		}
	}
	return respond.Error(ctx, contractshttp.StatusNotFound, "role not found")
}

// DeleteRole deletes a Role; its Members stop holding it.
func (c *Controller) DeleteRole(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "role not found")
	}
	if err := c.service.DeleteRole(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), id); err != nil {
		return roleFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

type orderRequest struct {
	RoleIDs []uint64 `json:"role_ids"`
}

// ReorderRoles places the Current guild's Roles in the order sent (top
// first, every Role but the Base role) and lists them as they are then.
func (c *Controller) ReorderRoles(ctx contractshttp.Context) contractshttp.Response {
	var req orderRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	if _, err := c.service.ReorderRoles(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), req.RoleIDs); err != nil {
		return roleFailure(ctx, err)
	}
	return c.Roles(ctx)
}

// AssignRole gives a Member of the Current guild a Role.
func (c *Controller) AssignRole(ctx contractshttp.Context) contractshttp.Response {
	return c.reRole(ctx, c.service.AssignRole)
}

// RemoveRole takes a Role from a Member of the Current guild.
func (c *Controller) RemoveRole(ctx contractshttp.Context) contractshttp.Response {
	return c.reRole(ctx, c.service.RemoveRole)
}

type reRoleFunc func(ctx context.Context, guildID, actorID uint64, perms domain.Permissions, memberID, roleID uint64) (domain.Membership, error)

func (c *Controller) reRole(ctx contractshttp.Context, change reRoleFunc) contractshttp.Response {
	memberID, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	roleID, err := strconv.ParseUint(ctx.Request().Route("role_id"), 10, 64)
	if err != nil {
		return respond.Error(ctx, contractshttp.StatusNotFound, "role not found")
	}
	ms, err := change(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), memberID, roleID)
	if err != nil {
		return roleFailure(ctx, err)
	}
	roles, err := c.service.RolesIn(ctx.Context(), ms.GuildID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	m, found, err := identity.MemberByID(ctx.Context(), ms.MemberID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if !found {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	return ctx.Response().Success().Json(contractshttp.Json{"member": memberWithRoles(m, roles, ms)})
}

// roleFailure answers the errors of managing Roles.
func roleFailure(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case errors.Is(err, domain.ErrInvalidRoleName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrInvalidRoleColor):
		return respond.Invalid(ctx, "color", err.Error())
	case errors.Is(err, domain.ErrUnknownPermission):
		return respond.Invalid(ctx, "permissions", err.Error())
	case errors.Is(err, domain.ErrInvalidOrder):
		return respond.Invalid(ctx, "role_ids", err.Error())
	case errors.Is(err, app.ErrRoleNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "role not found")
	case errors.Is(err, domain.ErrRoleNotBelow), errors.Is(err, domain.ErrBaseRoleFixed), errors.Is(err, domain.ErrNotHeld):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	}
	return membershipFailure(ctx, err)
}
