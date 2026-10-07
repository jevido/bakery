package http

import (
	"errors"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

type invitationJSON struct {
	ID    uint64    `json:"id"`
	Email string    `json:"email"`
	Roles []roleRef `json:"roles"`
	// Role is the former role the Roles read as, for older scripts.
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// invitationToJSON is i with the Roles it gives among its Guild's roles.
func invitationToJSON(i domain.Invitation, roles []domain.Role) invitationJSON {
	return invitationJSON{
		ID: i.ID, Email: i.Email, Roles: refs(roles, i.RoleIDs),
		Role:      wireRole(domain.PermissionsOf(roles, domain.Membership{RoleIDs: i.RoleIDs})),
		CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt,
	}
}

// invitationFailure answers the errors of Invitations.
func invitationFailure(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, identity.ErrInvalidEmail):
		return respond.Invalid(ctx, "email", err.Error())
	case errors.Is(err, domain.ErrInvalidRole):
		return respond.Invalid(ctx, "role", err.Error())
	case errors.Is(err, app.ErrRoleNotFound):
		return respond.Invalid(ctx, "role_ids", "no such role in this guild")
	case errors.Is(err, domain.ErrRoleNotBelow), errors.Is(err, domain.ErrBaseRoleFixed), errors.As(err, new(domain.ErrMissing)):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.Is(err, identity.ErrInvalidName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, identity.ErrPasswordTooShort):
		return respond.Invalid(ctx, "password", err.Error())
	case errors.Is(err, app.ErrAlreadyMember), errors.Is(err, app.ErrAlreadyInvited):
		return respond.Invalid(ctx, "email", err.Error())
	case errors.Is(err, app.ErrInvitationNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "invitation not found")
	case errors.Is(err, domain.ErrInvitationUsed), errors.Is(err, domain.ErrInvitationExpired), errors.Is(err, domain.ErrInvitationRevoked):
		return respond.Error(ctx, contractshttp.StatusGone, err.Error())
	case errors.Is(err, app.ErrNotInvited):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	}
	return respond.ServerError(ctx, err)
}

// Invitations lists the Current guild's open Invitations.
func (c *Controller) Invitations(ctx contractshttp.Context) contractshttp.Response {
	invs, err := c.service.OpenInvitations(ctx.Context(), Current(ctx))
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	roles, err := c.service.RolesIn(ctx.Context(), Current(ctx))
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]invitationJSON, len(invs))
	for i, inv := range invs {
		out[i] = invitationToJSON(inv, roles)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"invitations": out})
}

type inviteRequest struct {
	Email   string   `json:"email"`
	RoleIDs []uint64 `json:"role_ids"`
	// Role is the former role ("viewer", "member" or "admin"), read as
	// the seeded Role of that name when role_ids is absent, for older
	// scripts.
	Role string `json:"role"`
}

// Invite makes an Invitation into the Current guild and answers its link,
// which Invited may also email.
func (c *Controller) Invite(ctx contractshttp.Context) contractshttp.Response {
	var req inviteRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	guild := placeOf(ctx).guild
	actor := MemberID(ctx)
	roles, err := c.service.RolesIn(ctx.Context(), guild.ID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if req.RoleIDs == nil && req.Role != "" {
		r, found, err := domain.SeededRole(roles, req.Role)
		if err != nil {
			return invitationFailure(ctx, err)
		}
		if !found {
			return respond.Invalid(ctx, "role", "the "+req.Role+" role was renamed or deleted; send role_ids")
		}
		req.RoleIDs = []uint64{r.ID}
	}
	inv, token, err := c.service.Invite(ctx.Context(), guild.ID, actor, PermissionsOf(ctx), req.Email, req.RoleIDs)
	if err != nil {
		return invitationFailure(ctx, err)
	}
	ij := invitationToJSON(inv, roles)
	// The dashboard routes by hash.
	path := "/#/invite/" + token
	out := contractshttp.Json{"invitation": ij, "path": path}
	origin := dashboardOrigin(ctx)
	if origin != "" {
		out["link"] = origin + path
	}
	out["emailed"] = false
	if c.Invited != nil {
		if origin == "" {
			origin = "http://localhost:4930"
		}
		name := ""
		if m, found, err := identity.MemberByID(ctx.Context(), actor); err == nil && found {
			name = m.Name
		}
		names := make([]string, len(ij.Roles))
		for i, r := range ij.Roles {
			names[i] = r.Name
		}
		emailed, err := c.Invited(ctx.Context(), inv, guild, names, name, origin+path)
		out["emailed"] = emailed
		if err != nil {
			// The Invitation stands; its link can still be copied.
			out["email_error"] = err.Error()
		}
	}
	return ctx.Response().Json(contractshttp.StatusCreated, out)
}

// dashboardOrigin is where the dashboard is served: bakery.dashboard.url,
// else its own domain on a server, else the origin the request came from
// (the Vite dev server).
func dashboardOrigin(ctx contractshttp.Context) string {
	cfg := facades.Config()
	if u := strings.TrimRight(cfg.GetString("bakery.dashboard.url"), "/"); u != "" {
		return u
	}
	if d := cfg.GetString("bakery.dashboard.domain"); d != "" {
		return "https://" + d
	}
	return ctx.Request().Header("Origin")
}

// RevokeInvitation makes an Invitation of the Current guild stop working.
func (c *Controller) RevokeInvitation(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "invitation not found")
	}
	if err := c.service.RevokeInvitation(ctx.Context(), Current(ctx), id); err != nil {
		return invitationFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

// InvitationByToken is for the page an Invitation link opens; it needs no
// Session.
func (c *Controller) InvitationByToken(ctx contractshttp.Context) contractshttp.Response {
	in, err := c.service.InvitationByToken(ctx.Context(), ctx.Request().Route("token"))
	if err != nil {
		return invitationFailure(ctx, err)
	}
	roles, err := c.service.RolesIn(ctx.Context(), in.Guild.ID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	ij := invitationToJSON(in.Invitation, roles)
	return ctx.Response().Success().Json(contractshttp.Json{
		"invitation": contractshttp.Json{
			"email": ij.Email, "roles": ij.Roles, "role": ij.Role, "expires_at": ij.ExpiresAt,
		},
		"guild":           contractshttp.Json{"name": in.Guild.Name},
		"existing_member": in.ExistingMember,
	})
}

// DeclineInvitation turns an Invitation down through its link; like the
// page itself it needs no Session.
func (c *Controller) DeclineInvitation(ctx contractshttp.Context) contractshttp.Response {
	if err := c.service.DeclineInvitation(ctx.Context(), ctx.Request().Route("token")); err != nil {
		return invitationFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

type acceptRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// AcceptInvitation gives the invited person the Membership and makes its
// Guild their Current guild. A new email gets a Member (with the name and
// password sent) and is signed in, as Setup does; an existing Member's email
// needs that Member's Session.
func (c *Controller) AcceptInvitation(ctx contractshttp.Context) contractshttp.Response {
	token := ctx.Request().Route("token")
	in, err := c.service.InvitationByToken(ctx.Context(), token)
	if err != nil {
		return invitationFailure(ctx, err)
	}
	if in.ExistingMember {
		return c.acceptAsMember(ctx, token, in.Invitation)
	}
	var req acceptRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	inv, memberID, err := c.service.AcceptAsNewMember(ctx.Context(), token, req.Name, req.Password)
	if errors.Is(err, app.ErrSignInToAccept) {
		return signInToAccept(ctx, in.Invitation.Email)
	}
	if err != nil {
		return invitationFailure(ctx, err)
	}
	if err := identity.SignIn(ctx, memberID); err != nil {
		return respond.ServerError(ctx, err)
	}
	return c.accepted(ctx, contractshttp.StatusCreated, inv, memberID)
}

func (c *Controller) acceptAsMember(ctx contractshttp.Context, token string, inv domain.Invitation) contractshttp.Response {
	m, signedIn, err := identity.SessionMember(ctx)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if !signedIn {
		return signInToAccept(ctx, inv.Email)
	}
	if inv, err = c.service.AcceptAsMember(ctx.Context(), token, m.ID, m.Email); err != nil {
		return invitationFailure(ctx, err)
	}
	return c.accepted(ctx, contractshttp.StatusOK, inv, m.ID)
}

func signInToAccept(ctx contractshttp.Context, email string) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusUnauthorized, "sign in as "+email+" to accept")
}

// accepted answers the Member who accepted inv, with their former role in its
// Guild, and makes that Guild their Current guild.
func (c *Controller) accepted(ctx contractshttp.Context, status int, inv domain.Invitation, memberID uint64) contractshttp.Response {
	m, found, err := identity.MemberByID(ctx.Context(), memberID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if !found {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	perms, _, err := c.service.PermissionsIn(ctx.Context(), inv.GuildID, memberID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	SetCurrent(ctx, inv.GuildID)
	return ctx.Response().Json(status, contractshttp.Json{"member": toJSON(m, wireRole(perms))})
}
