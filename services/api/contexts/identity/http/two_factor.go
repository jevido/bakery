package http

import (
	"errors"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// twoFactorError answers the errors every two-factor route shares, or nil.
func twoFactorError(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrWrongCode), errors.Is(err, domain.ErrCodeUsed):
		return respond.Invalid(ctx, "code", err.Error())
	case errors.Is(err, app.ErrBadCredentials):
		return respond.Invalid(ctx, "password", "the password is wrong")
	case errors.Is(err, app.ErrTwoFactorOn), errors.Is(err, app.ErrTwoFactorOff), errors.Is(err, app.ErrTwoFactorNotPending):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case errors.Is(err, app.ErrMemberNotFound):
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in")
	}
	return respond.ServerError(ctx, err)
}

func (c *Controller) TwoFactor(ctx contractshttp.Context) contractshttp.Response {
	id, _ := MemberID(ctx)
	st, err := c.service.TwoFactorStatus(ctx.Context(), id)
	if r := twoFactorError(ctx, err); r != nil {
		return r
	}
	return ctx.Response().Success().Json(contractshttp.Json{"state": st.State, "recovery_codes_left": st.RecoveryCodesLeft})
}

func (c *Controller) StartTwoFactor(ctx contractshttp.Context) contractshttp.Response {
	id, _ := MemberID(ctx)
	setup, err := c.service.StartTwoFactor(ctx.Context(), id)
	if r := twoFactorError(ctx, err); r != nil {
		return r
	}
	return ctx.Response().Success().Json(contractshttp.Json{"secret": setup.Secret, "otpauth_uri": setup.OTPAuthURI})
}

type codeRequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
	Password     string `json:"password"`
}

func (c *Controller) ConfirmTwoFactor(ctx contractshttp.Context) contractshttp.Response {
	var req codeRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	codes, err := c.service.ConfirmTwoFactor(ctx.Context(), id, req.Code)
	if r := twoFactorError(ctx, err); r != nil {
		return r
	}
	return ctx.Response().Success().Json(contractshttp.Json{"recovery_codes": codes})
}

func (c *Controller) RegenerateRecoveryCodes(ctx contractshttp.Context) contractshttp.Response {
	var req codeRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	codes, err := c.service.RegenerateRecoveryCodes(ctx.Context(), id, req.Code)
	if r := twoFactorError(ctx, err); r != nil {
		return r
	}
	return ctx.Response().Success().Json(contractshttp.Json{"recovery_codes": codes})
}

func (c *Controller) DisableTwoFactor(ctx contractshttp.Context) contractshttp.Response {
	var req codeRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	err := c.service.DisableTwoFactor(ctx.Context(), id, req.Password, req.Code, req.RecoveryCode)
	if r := twoFactorError(ctx, err); r != nil {
		return r
	}
	return ctx.Response().NoContent()
}
