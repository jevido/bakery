package http

import (
	"errors"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// profileError answers the errors the Profile routes share, or nil.
func profileError(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrInvalidName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrPasswordTooShort):
		return respond.Invalid(ctx, "new_password", err.Error())
	case errors.Is(err, app.ErrBadCredentials):
		// 422, not 401: the dashboard reads every 401 as "signed out".
		return respond.Invalid(ctx, "current_password", "the password is wrong")
	case errors.Is(err, app.ErrMemberNotFound):
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in")
	}
	return respond.ServerError(ctx, err)
}

type nameRequest struct {
	Name string `json:"name"`
}

func (c *Controller) ChangeName(ctx contractshttp.Context) contractshttp.Response {
	var req nameRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	m, err := c.service.ChangeName(ctx.Context(), id, req.Name)
	if r := profileError(ctx, err); r != nil {
		return r
	}
	return ctx.Response().Success().Json(contractshttp.Json{"member": toJSON(m)})
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword ends every other Session and gives this browser a new
// one.
func (c *Controller) ChangePassword(ctx contractshttp.Context) contractshttp.Response {
	var req passwordRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	m, err := c.service.ChangePassword(ctx.Context(), id, req.CurrentPassword, req.NewPassword)
	if r := profileError(ctx, err); r != nil {
		return r
	}
	return c.withSession(ctx, contractshttp.StatusOK, m)
}

// SignOutOtherSessions ends every other Session and gives this browser a
// new one.
func (c *Controller) SignOutOtherSessions(ctx contractshttp.Context) contractshttp.Response {
	id, _ := MemberID(ctx)
	m, err := c.service.SignOutOtherSessions(ctx.Context(), id)
	if r := profileError(ctx, err); r != nil {
		return r
	}
	return c.withSession(ctx, contractshttp.StatusOK, m)
}
