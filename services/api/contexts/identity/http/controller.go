// Package http exposes identity over HTTP: setup, login, logout, me, and the
// Auth middleware every other route sits behind.
package http

import (
	"errors"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// SessionCookie carries the Session: a JWT the dashboard's scripts cannot
// read. A cookie, not a header, because EventSource cannot send headers.
const SessionCookie = "bakery_session"

type ownerKey struct{}

type Controller struct {
	service *app.Service
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

type ownerJSON struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func toJSON(o domain.Owner) ownerJSON {
	return ownerJSON{ID: o.ID, Name: o.Name, Email: o.Email}
}

func (c *Controller) SetupStatus(ctx contractshttp.Context) contractshttp.Response {
	needed, err := c.service.SetupNeeded(ctx.Context())
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"needed": needed})
}

type setupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *Controller) Setup(ctx contractshttp.Context) contractshttp.Response {
	var req setupRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	owner, err := c.service.SetupOwner(ctx.Context(), req.Name, req.Email, req.Password)
	switch {
	case errors.Is(err, domain.ErrInvalidName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrInvalidEmail):
		return respond.Invalid(ctx, "email", err.Error())
	case errors.Is(err, domain.ErrPasswordTooShort):
		return respond.Invalid(ctx, "password", err.Error())
	case errors.Is(err, app.ErrOwnerExists):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	return c.withSession(ctx, contractshttp.StatusCreated, owner)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *Controller) Login(ctx contractshttp.Context) contractshttp.Response {
	var req loginRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	owner, err := c.service.Login(ctx.Context(), req.Email, req.Password)
	if errors.Is(err, app.ErrBadCredentials) {
		return respond.Error(ctx, contractshttp.StatusUnauthorized, err.Error())
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return c.withSession(ctx, contractshttp.StatusOK, owner)
}

func (c *Controller) Logout(ctx contractshttp.Context) contractshttp.Response {
	ctx.Response().Cookie(sessionCookie("", -1))
	return ctx.Response().NoContent()
}

func (c *Controller) Me(ctx contractshttp.Context) contractshttp.Response {
	id, _ := OwnerID(ctx)
	owner, err := c.service.CurrentOwner(ctx.Context(), id)
	if errors.Is(err, app.ErrOwnerNotFound) {
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in")
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"owner": toJSON(owner)})
}

func (c *Controller) withSession(ctx contractshttp.Context, status int, o domain.Owner) contractshttp.Response {
	token, err := facades.Auth(ctx).LoginUsingID(o.ID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	ctx.Response().Cookie(sessionCookie(token, facades.Config().GetInt("jwt.ttl")*60))
	return ctx.Response().Json(status, contractshttp.Json{"owner": toJSON(o)})
}

// sessionCookie is host-only and SameSite=Strict: the dashboard reaches the
// API on its own origin (the Vite proxy in dev), so the cookie never needs to
// travel cross-site, and a form on another site cannot use it.
func sessionCookie(value string, maxAge int) contractshttp.Cookie {
	return contractshttp.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/api",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   facades.Config().GetString("app.env") == "production",
		SameSite: "strict",
	}
}

// Auth lets a request through only with a valid Session, and puts the
// Owner's id on the context for OwnerID.
type Auth struct{}

func (Auth) Signature() string { return "identity.auth" }

func (Auth) Handle(ctx contractshttp.Context) {
	token := ctx.Request().Cookie(SessionCookie)
	if token == "" {
		_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in").Abort()
		return
	}
	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil {
		_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in").Abort()
		return
	}
	id, err := strconv.ParseUint(payload.Key, 10, 64)
	if err != nil {
		_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in").Abort()
		return
	}
	ctx.WithValue(ownerKey{}, id)
	ctx.Request().Next()
}

// OwnerID returns the id of the Owner Auth let through.
func OwnerID(ctx contractshttp.Context) (uint64, bool) {
	id, ok := ctx.Value(ownerKey{}).(uint64)
	return id, ok
}
