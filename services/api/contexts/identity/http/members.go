package http

import (
	"errors"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type invitationJSON struct {
	ID        uint64    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func invitationToJSON(i domain.Invitation) invitationJSON {
	return invitationJSON{ID: i.ID, Email: i.Email, Role: string(i.Role), CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt}
}

// memberFailure answers the errors of managing Members and Invitations.
func memberFailure(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail):
		return respond.Invalid(ctx, "email", err.Error())
	case errors.Is(err, domain.ErrInvitationRole), errors.Is(err, domain.ErrInvalidRole), errors.Is(err, domain.ErrGrantOwner):
		return respond.Invalid(ctx, "role", err.Error())
	case errors.Is(err, domain.ErrInvalidName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrPasswordTooShort):
		return respond.Invalid(ctx, "password", err.Error())
	case errors.Is(err, app.ErrAlreadyMember), errors.Is(err, app.ErrAlreadyInvited):
		return respond.Invalid(ctx, "email", err.Error())
	case errors.Is(err, domain.ErrNotAdmin), errors.Is(err, domain.ErrOwnerIsFixed), errors.Is(err, domain.ErrSelf):
		return respond.Error(ctx, contractshttp.StatusForbidden, err.Error())
	case errors.Is(err, app.ErrMemberNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	case errors.Is(err, app.ErrInvitationNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, "invitation not found")
	case errors.Is(err, domain.ErrInvitationUsed), errors.Is(err, domain.ErrInvitationExpired), errors.Is(err, domain.ErrInvitationRevoked):
		return respond.Error(ctx, contractshttp.StatusGone, err.Error())
	}
	return respond.ServerError(ctx, err)
}

func routeID(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func (c *Controller) Members(ctx contractshttp.Context) contractshttp.Response {
	members, err := c.service.AllMembers(ctx.Context())
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]memberJSON, len(members))
	for i, m := range members {
		out[i] = toJSON(m)
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
	role, err := domain.ParseRole(req.Role)
	if err != nil {
		return memberFailure(ctx, err)
	}
	actor, _ := MemberID(ctx)
	m, err := c.service.ChangeRole(ctx.Context(), actor, id, role)
	if err != nil {
		return memberFailure(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"member": toJSON(m)})
}

func (c *Controller) RemoveMember(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "member not found")
	}
	actor, _ := MemberID(ctx)
	if err := c.service.RemoveMember(ctx.Context(), actor, id); err != nil {
		return memberFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

func (c *Controller) Invitations(ctx contractshttp.Context) contractshttp.Response {
	invs, err := c.service.OpenInvitations(ctx.Context())
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]invitationJSON, len(invs))
	for i, inv := range invs {
		out[i] = invitationToJSON(inv)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"invitations": out})
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (c *Controller) Invite(ctx contractshttp.Context) contractshttp.Response {
	var req inviteRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	actor, _ := MemberID(ctx)
	inv, token, err := c.service.Invite(ctx.Context(), actor, req.Email, domain.Role(req.Role))
	if err != nil {
		return memberFailure(ctx, err)
	}
	// The dashboard routes by hash.
	path := "/#/invite/" + token
	out := contractshttp.Json{"invitation": invitationToJSON(inv), "path": path}
	if origin := dashboardOrigin(ctx); origin != "" {
		out["link"] = origin + path
	}
	return ctx.Response().Json(contractshttp.StatusCreated, out)
}

// dashboardOrigin is where the dashboard is served: its own domain on a
// server, else the origin the request came from (the Vite dev server).
func dashboardOrigin(ctx contractshttp.Context) string {
	if d := facades.Config().GetString("bakery.dashboard.domain"); d != "" {
		return "https://" + d
	}
	return ctx.Request().Header("Origin")
}

func (c *Controller) RevokeInvitation(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "invitation not found")
	}
	if err := c.service.RevokeInvitation(ctx.Context(), id); err != nil {
		return memberFailure(ctx, err)
	}
	return ctx.Response().NoContent()
}

// InvitationByToken is for the page an Invitation link opens; it needs no
// Session.
func (c *Controller) InvitationByToken(ctx contractshttp.Context) contractshttp.Response {
	inv, err := c.service.InvitationByToken(ctx.Context(), ctx.Request().Route("token"))
	if err != nil {
		return memberFailure(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"invitation": contractshttp.Json{
		"email": inv.Email, "role": string(inv.Role), "expires_at": inv.ExpiresAt,
	}})
}

type acceptRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// AcceptInvitation creates the Member and signs them in, as Setup does.
func (c *Controller) AcceptInvitation(ctx contractshttp.Context) contractshttp.Response {
	var req acceptRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	m, err := c.service.AcceptInvitation(ctx.Context(), ctx.Request().Route("token"), req.Name, req.Password)
	if err != nil {
		return memberFailure(ctx, err)
	}
	return c.withSession(ctx, contractshttp.StatusCreated, m)
}
