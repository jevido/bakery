// Package http exposes identity over HTTP: setup, login, logout, me, and the
// Auth middleware every other route sits behind.
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

// principal is who a request comes from: a Member and the Role it acts with.
type principal struct {
	memberID uint64
	role     domain.Role
	// viaAPIToken is set when an API token, not a Session, authenticated
	// the request.
	viaAPIToken bool
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

type memberJSON struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
	// TwoFactor is whether the Member's Two-factor authentication is on.
	TwoFactor bool `json:"two_factor"`
}

func toJSON(m domain.Member) memberJSON {
	return memberJSON{ID: m.ID, Name: m.Name, Email: m.Email, Role: string(m.Role), TwoFactor: m.TwoFactor.On()}
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

func (c *Controller) Me(ctx contractshttp.Context) contractshttp.Response {
	id, _ := MemberID(ctx)
	m, err := c.service.CurrentMember(ctx.Context(), id)
	if errors.Is(err, app.ErrMemberNotFound) {
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in")
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	m.Role = RoleOf(ctx)
	return ctx.Response().Success().Json(contractshttp.Json{"member": toJSON(m)})
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

// Auth lets a request through only from a Member, by Session cookie or by
// an API token in the Authorization header, and puts the principal on the
// context for MemberID, RoleOf and the other middlewares. The Member is read
// on every request, so a changed Role or a removed Member counts at once. A
// viewer is refused anything but reading.
type Auth struct {
	Service *app.Service
	// SelfService is for the routes where a Member manages their own API
	// tokens, Account and two-factor: every Role may change those, but only
	// with a Session, so a leaked token can neither mint more nor switch
	// two-factor off.
	SelfService bool
}

func (Auth) Signature() string { return "identity.auth" }

func (a Auth) Handle(ctx contractshttp.Context) {
	p, ok := a.principal(ctx)
	if !ok {
		return
	}
	if a.SelfService {
		if p.viaAPIToken {
			_ = respond.Error(ctx, contractshttp.StatusForbidden, "this needs a signed-in session, not an API token").Abort()
			return
		}
	} else if reason := refusal(ctx.Request().Method(), p.role); reason != "" {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, reason).Abort()
		return
	}
	ctx.WithValue(principalKey{}, p)
	ctx.Request().Next()
}

// principal authenticates the request, answering 401 itself when it cannot.
func (a Auth) principal(ctx contractshttp.Context) (principal, bool) {
	unauthorized := func() (principal, bool) {
		_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in").Abort()
		return principal{}, false
	}
	if header := ctx.Request().Header("Authorization"); header != "" {
		value, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "invalid API token").Abort()
			return principal{}, false
		}
		m, role, err := a.Service.Authenticate(ctx.Context(), strings.TrimSpace(value))
		if errors.Is(err, app.ErrInvalidAPIToken) {
			_ = respond.Error(ctx, contractshttp.StatusUnauthorized, "invalid API token").Abort()
			return principal{}, false
		}
		if err != nil {
			_ = respond.ServerError(ctx, err).Abort()
			return principal{}, false
		}
		return principal{memberID: m.ID, role: role, viaAPIToken: true}, true
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
	m, err := a.Service.CurrentMember(ctx.Context(), id)
	if errors.Is(err, app.ErrMemberNotFound) {
		return unauthorized()
	}
	if err != nil {
		_ = respond.ServerError(ctx, err).Abort()
		return principal{}, false
	}
	return principal{memberID: m.ID, role: m.Role}, true
}

// refusal is why a Role may not make a request with this method, or "".
func refusal(method string, role domain.Role) string {
	if method == contractshttp.MethodGet || method == contractshttp.MethodHead || role.CanWrite() {
		return ""
	}
	return "your role cannot change this"
}

// Admin lets only admins and the Owner through. It runs after Auth.
type Admin struct{}

func (Admin) Signature() string { return "identity.admin" }

func (Admin) Handle(ctx contractshttp.Context) {
	if !RoleOf(ctx).IsAdmin() {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "only admins can do this").Abort()
		return
	}
	ctx.Request().Next()
}

// Secrets keeps viewers away from routes that return Secrets. It runs after
// Auth.
type Secrets struct{}

func (Secrets) Signature() string { return "identity.secrets" }

func (Secrets) Handle(ctx contractshttp.Context) {
	if !RoleOf(ctx).CanSeeSecrets() {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "your role cannot see secrets").Abort()
		return
	}
	ctx.Request().Next()
}

// MemberID returns the id of the Member Auth let through.
func MemberID(ctx contractshttp.Context) (uint64, bool) {
	p, ok := ctx.Value(principalKey{}).(principal)
	return p.memberID, ok
}

// RoleOf returns the Role the request acts with ("" before Auth ran).
func RoleOf(ctx contractshttp.Context) domain.Role {
	p, _ := ctx.Value(principalKey{}).(principal)
	return p.role
}
