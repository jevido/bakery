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
	// names of the Roles it gives, the inviting Member's name and its link,
	// and answers whether it emailed the link.
	Invited func(ctx context.Context, inv domain.Invitation, guild domain.Guild, roles []string, invitedBy, link string) (emailed bool, err error)
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
	// GuildMaster is set for the Current guild's Guild Master.
	GuildMaster bool `json:"guild_master"`
	// Roles are the Roles they hold in the Current guild besides the Base
	// role, top first; only in the Members list and its changes.
	Roles *[]roleRef `json:"roles,omitempty"`
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
	// Permissions are the wire keys of the Member's Permissions there.
	Permissions []string `json:"permissions"`
}

// Me is the signed-in Member with their Permissions (and former role) in
// the Current guild and the Guilds they may switch to.
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
		current = &guildJSON{ID: p.guild.ID, Name: p.guild.Name, Permissions: p.permissions.Keys()}
	}
	offers, err := c.offersTo(ctx, m.ID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	master := p.guild.ID != 0 && p.guild.MasterID == m.ID
	me := toJSON(m, wireRole(p.permissions))
	me.GuildMaster = master
	return ctx.Response().Success().Json(contractshttp.Json{
		"member":         me,
		"role":           wireRole(p.permissions),
		"permissions":    p.permissions.Keys(),
		"instance_admin": m.InstanceAdmin,
		"guild_master":   master,
		"offers":         offers,
		"guild":          current,
		"guilds":         guilds,
	})
}

// Members lists the Members of the Current guild, its Guild Master first,
// then the Instance admin, then by name, each with the Roles they hold.
func (c *Controller) Members(ctx contractshttp.Context) contractshttp.Response {
	ms, err := c.service.MembershipsIn(ctx.Context(), Current(ctx))
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	guildRoles, err := c.service.RolesIn(ctx.Context(), Current(ctx))
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	held := make(map[uint64]domain.Membership, len(ms))
	ids := make([]uint64, len(ms))
	for i, m := range ms {
		held[m.MemberID], ids[i] = m, m.MemberID
	}
	members, err := identity.Members(ctx.Context(), ids)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	master := placeOf(ctx).guild.MasterID
	out := make([]memberJSON, 0, len(members))
	for _, m := range members {
		j := memberWithRoles(m, guildRoles, held[m.ID])
		if m.ID == master {
			j.GuildMaster = true
			out = append([]memberJSON{j}, out...)
			continue
		}
		out = append(out, j)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"members": out})
}

// memberWithRoles is m with their former role and the Roles their
// Membership holds among the Guild's roles.
func memberWithRoles(m identity.Member, roles []domain.Role, ms domain.Membership) memberJSON {
	j := toJSON(m, wireRole(domain.PermissionsOf(roles, ms)))
	held := refs(roles, ms.RoleIDs)
	j.Roles = &held
	return j
}

// ChangeRoleGone answers the former `PATCH /api/members/{id}`: a Member
// holds several Roles now.
func (c *Controller) ChangeRoleGone(ctx contractshttp.Context) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusGone,
		"a member holds several roles now: use PUT or DELETE /api/members/{id}/roles/{role_id}")
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
	case errors.Is(err, domain.ErrGuildMaster):
		return respond.Error(ctx, contractshttp.StatusForbidden, "the guild master's two-factor can only be reset on the server")
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
	case errors.As(err, new(domain.ErrMissing)), errors.Is(err, domain.ErrInstanceAdminFixed), errors.Is(err, domain.ErrSelf),
		errors.Is(err, domain.ErrGuildMaster), errors.Is(err, domain.ErrMemberNotBelow):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.Is(err, app.ErrMembershipNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	return respond.ServerError(ctx, err)
}

func routeID(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}
