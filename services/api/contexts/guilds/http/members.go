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

type Controller struct {
	service *app.Service
	// Invited, when set, hears of each new Invitation with its Guild, the
	// inviting Member's name and its link, and answers whether it emailed
	// the link.
	Invited func(ctx context.Context, inv domain.Invitation, guild domain.Guild, invitedBy, link string) (emailed bool, err error)
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

// ownerRole is what the Instance admin's Role reads as on the wire: the
// dashboard still knows them as the Owner, whom nobody manages.
const ownerRole = "owner"

type memberJSON struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	// Role is the former role in the Current guild (wireRole), "owner"
	// for the Instance admin.
	Role          string `json:"role"`
	TwoFactor     bool   `json:"two_factor"`
	InstanceAdmin bool   `json:"instance_admin"`
}

func toJSON(m identity.Member, role string) memberJSON {
	r := role
	if m.InstanceAdmin {
		r = ownerRole
	}
	return memberJSON{ID: m.ID, Name: m.Name, Email: m.Email, Role: r, TwoFactor: m.TwoFactor, InstanceAdmin: m.InstanceAdmin}
}

type guildJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// Me is the signed-in Member with their former role in the Current guild and the
// Guilds they may switch to.
func (c *Controller) Me(ctx contractshttp.Context) contractshttp.Response {
	p := placeOf(ctx)
	m, found, err := identity.MemberByID(ctx.Context(), p.principal.MemberID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if !found {
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in")
	}
	places, err := c.service.GuildsFor(ctx.Context(), m.ID, m.InstanceAdmin)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	guilds := guildsJSON(places)
	var current *guildJSON
	if p.guild.ID != 0 {
		current = &guildJSON{ID: p.guild.ID, Name: p.guild.Name}
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"member":         toJSON(m, wireRole(p.permissions)),
		"role":           wireRole(p.permissions),
		"instance_admin": m.InstanceAdmin,
		"guild":          current,
		"guilds":         guilds,
	})
}

// Members lists the Members of the Current guild, the Instance admin first,
// then by name.
func (c *Controller) Members(ctx contractshttp.Context) contractshttp.Response {
	ms, err := c.service.MembershipsIn(ctx.Context(), Current(ctx))
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	guildRoles, err := c.service.RolesIn(ctx.Context(), Current(ctx))
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	roles := make(map[uint64]string, len(ms))
	ids := make([]uint64, len(ms))
	for i, m := range ms {
		roles[m.MemberID], ids[i] = wireRole(domain.PermissionsOf(guildRoles, m)), m.MemberID
	}
	members, err := identity.Members(ctx.Context(), ids)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]memberJSON, len(members))
	for i, m := range members {
		out[i] = toJSON(m, roles[m.ID])
	}
	return ctx.Response().Success().Json(contractshttp.Json{"members": out})
}

type roleRequest struct {
	Role string `json:"role"`
}

func (c *Controller) ChangeRole(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	var req roleRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	ms, err := c.service.ChangeRole(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), id, req.Role)
	if err != nil {
		return membershipFailure(ctx, err)
	}
	perms, _, err := c.service.PermissionsIn(ctx.Context(), ms.GuildID, ms.MemberID)
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
	return ctx.Response().Success().Json(contractshttp.Json{"member": toJSON(m, wireRole(perms))})
}

// RemoveMember takes the Member out of the Current guild.
func (c *Controller) RemoveMember(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	if err := c.service.RemoveMembership(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), id); err != nil {
		return membershipFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

// ResetTwoFactor switches the two-factor of a locked-out Member of the
// Current guild off.
func (c *Controller) ResetTwoFactor(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	err := c.service.ResetTwoFactor(ctx.Context(), Current(ctx), MemberID(ctx), PermissionsOf(ctx), id)
	switch {
	case errors.Is(err, domain.ErrInstanceAdminFixed):
		return respond.Error(ctx, contractshttp.StatusForbidden, "the instance admin's two-factor can only be reset on the server")
	case errors.Is(err, domain.ErrSelf):
		return respond.Error(ctx, contractshttp.StatusForbidden, "switch your own two-factor off on your Profile page")
	case errors.Is(err, identity.ErrTwoFactorOff):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case errors.Is(err, identity.ErrMemberNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	case err != nil:
		return membershipFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

// membershipFailure answers the errors of managing Memberships.
func membershipFailure(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case errors.Is(err, domain.ErrInvalidRole):
		return respond.Invalid(ctx, "role", err.Error())
	case errors.Is(err, domain.ErrNotAdmin), errors.Is(err, domain.ErrInstanceAdminFixed), errors.Is(err, domain.ErrSelf):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrLastAdmin):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case errors.Is(err, app.ErrMembershipNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	return respond.ServerError(ctx, err)
}

func routeID(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}
