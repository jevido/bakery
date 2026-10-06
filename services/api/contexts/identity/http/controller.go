// Package http exposes identity over HTTP: setup, login, logout, Profile,
// two-factor, API tokens and Invitations, and Authenticate, which guilds'
// middlewares (and with them every other route) are built on.
package http

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// SessionCookie carries the Session: a JWT the dashboard's scripts cannot
// read. A cookie, not a header, because EventSource cannot send headers.
const SessionCookie = "bakery_session"

type principalKey struct{}

// Principal is who a request comes from.
type Principal struct {
	MemberID      uint64
	InstanceAdmin bool
	// Token is the API token that authenticated the request, nil for a
	// Session (which may do everything its Role may).
	Token *domain.APIToken
}

// Allows reports whether the request's API token, if any, carries p.
func (p Principal) Allows(perm domain.Permission) bool {
	return p.Token == nil || p.Token.Allows(perm)
}

type guildKey struct{}

// place is the Guild a request acts in and the Member's Role there, as
// guilds told it with ActIn.
type place struct {
	guildID uint64
	role    domain.Role
}

type Controller struct {
	service *app.Service
	// Invited, when set, hears of each new Invitation with its link and the
	// inviting Member's name, and answers whether it emailed the link.
	Invited func(ctx context.Context, inv domain.Invitation, invitedBy, link string) (emailed bool, err error)
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

// memberJSON is a Member as sign-in and the Profile answer it. The Role is
// per Guild, so it is not here: guilds' `GET /api/me` has it.
type memberJSON struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	// TwoFactor is whether the Member's Two-factor authentication is on.
	TwoFactor bool `json:"two_factor"`
}

func toJSON(m domain.Member) memberJSON {
	return memberJSON{ID: m.ID, Name: m.Name, Email: m.Email, TwoFactor: m.TwoFactor.On()}
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
	admin, err := c.service.SetUp(ctx.Context(), req.Name, req.Email, req.Password)
	switch {
	case errors.Is(err, domain.ErrInvalidName):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrInvalidEmail):
		return respond.Invalid(ctx, "email", err.Error())
	case errors.Is(err, domain.ErrPasswordTooShort):
		return respond.Invalid(ctx, "password", err.Error())
	case errors.Is(err, app.ErrSetupDone):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	return c.withSession(ctx, contractshttp.StatusCreated, admin)
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
	m, err := c.service.Login(ctx.Context(), req.Email, req.Password)
	if errors.Is(err, app.ErrBadCredentials) {
		return respond.Error(ctx, contractshttp.StatusUnauthorized, err.Error())
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if m.TwoFactor.On() {
		// No Session yet: the Login challenge waits for the second step.
		if err := setLoginChallenge(ctx, loginChallenge{MemberID: m.ID, ExpiresAt: time.Now().Add(loginChallengeTTL)}); err != nil {
			return respond.ServerError(ctx, err)
		}
		return ctx.Response().Success().Json(contractshttp.Json{"two_factor_required": true})
	}
	return c.withSession(ctx, contractshttp.StatusOK, m)
}

func (c *Controller) Logout(ctx contractshttp.Context) contractshttp.Response {
	ctx.Response().Cookie(sessionCookie("", -1))
	return ctx.Response().NoContent()
}

func (c *Controller) withSession(ctx contractshttp.Context, status int, m domain.Member) contractshttp.Response {
	token, err := facades.Auth(ctx).LoginUsingID(m.ID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	ctx.Response().Cookie(sessionCookie(token, facades.Config().GetInt("jwt.ttl")*60))
	return ctx.Response().Json(status, contractshttp.Json{"member": toJSON(m)})
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

// Authenticate finds the Member a request comes from, by an API token in
// the Authorization header or else the Session cookie, and puts the
// Principal on the context for MemberID. It answers 401 itself when there is
// none. The Member is read on every request, so a removed Member counts at
// once; an expired or unknown token is 401 alike.
func Authenticate(service *app.Service, ctx contractshttp.Context) (Principal, bool) {
	p, ok := authenticate(service, ctx)
	if ok {
		ctx.WithValue(principalKey{}, p)
	}
	return p, ok
}

func authenticate(service *app.Service, ctx contractshttp.Context) (Principal, bool) {
	unauthorized := func() (Principal, bool) {
		_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in").Abort()
		return Principal{}, false
	}
	if header := ctx.Request().Header("Authorization"); header != "" {
		value, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "invalid API token").Abort()
			return Principal{}, false
		}
		m, t, err := service.Authenticate(ctx.Context(), strings.TrimSpace(value))
		if errors.Is(err, app.ErrInvalidAPIToken) {
			_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "invalid API token").Abort()
			return Principal{}, false
		}
		if err != nil {
			_ = respond.ServerError(ctx, err).Abort()
			return Principal{}, false
		}
		return Principal{MemberID: m.ID, InstanceAdmin: m.InstanceAdmin, Token: &t}, true
	}
	token := ctx.Request().Cookie(SessionCookie)
	if token == "" {
		return unauthorized()
	}
	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil {
		return unauthorized()
	}
	id, err := strconv.ParseUint(payload.Key, 10, 64)
	if err != nil {
		return unauthorized()
	}
	m, err := service.CurrentMember(ctx.Context(), id)
	if errors.Is(err, app.ErrMemberNotFound) {
		return unauthorized()
	}
	if err != nil {
		_ = respond.ServerError(ctx, err).Abort()
		return Principal{}, false
	}
	// A password change or "sign out everywhere else" ended it.
	if !m.SessionCounts(payload.IssuedAt) {
		return unauthorized()
	}
	return Principal{MemberID: m.ID, InstanceAdmin: m.InstanceAdmin}, true
}

// Auth is for the routes where a Member manages their own Profile and
// two-factor (and, behind guilds' middleware, API tokens): every Role may
// change those, but only with a Session, so a leaked token can neither
// mint more nor switch two-factor off.
type Auth struct {
	Service *app.Service
}

func (Auth) Signature() string { return "identity.auth" }

func (a Auth) Handle(ctx contractshttp.Context) {
	p, ok := Authenticate(a.Service, ctx)
	if !ok {
		return
	}
	if p.Token != nil {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "this needs a signed-in session, not an API token").Abort()
		return
	}
	ctx.Request().Next()
}

// MemberID returns the id of the Member Authenticate let through.
func MemberID(ctx contractshttp.Context) (uint64, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p.MemberID, ok
}

// ActIn records the Guild the request acts in and the Member's Role there,
// for the routes identity serves inside a Guild (API tokens, Invitations).
func ActIn(ctx contractshttp.Context, guildID uint64, role domain.Role) {
	ctx.WithValue(guildKey{}, place{guildID: guildID, role: role})
}

// guildOf is what ActIn recorded; false when nothing did.
func guildOf(ctx contractshttp.Context) (place, bool) {
	p, ok := ctx.Value(guildKey{}).(place)
	return p, ok && p.guildID != 0
}
