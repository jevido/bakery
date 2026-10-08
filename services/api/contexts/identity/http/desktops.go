package http

import (
	"errors"
	"fmt"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// desktopSignInPollMs is how often the Desktop app is told to poll.
const desktopSignInPollMs = 1000

type startDesktopSignInRequest struct {
	ClientName string `json:"client_name"`
	// Token is the secret the app minted; it goes in the approve link.
	Token string `json:"token"`
	// DesktopKeyHash is the SHA-256, in hex, of the Desktop key the app
	// minted and keeps; the key itself never travels.
	DesktopKeyHash string `json:"desktop_key_hash"`
}

func desktopSignInPath(id uint64, token string) string {
	return fmt.Sprintf("/#/desktop-sign-in/%d?token=%s", id, token)
}

// requestBaseURL is the scheme and host the request reached The Bakery on:
// the dashboard's own address, also behind the Vite proxy or Caddy.
func requestBaseURL(ctx contractshttp.Context) string {
	r := ctx.Request().Origin()
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

func (c *Controller) StartDesktopSignIn(ctx contractshttp.Context) contractshttp.Response {
	var req startDesktopSignInRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	in, err := c.service.StartDesktopSignIn(ctx.Context(), req.ClientName, req.Token, req.DesktopKeyHash)
	switch {
	case errors.Is(err, domain.ErrInvalidDesktopName):
		return respond.Invalid(ctx, "client_name", err.Error())
	case errors.Is(err, domain.ErrInvalidDesktopSignInKey):
		return respond.Invalid(ctx, "token", err.Error())
	case errors.Is(err, domain.ErrInvalidDesktopKeyHash):
		return respond.Invalid(ctx, "desktop_key_hash", err.Error())
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	path := desktopSignInPath(in.ID, req.Token)
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{
		"id":               in.ID,
		"approval_path":    path,
		"approval_url":     requestBaseURL(ctx) + path,
		"expires_at":       in.ExpiresAt.UTC(),
		"poll_interval_ms": desktopSignInPollMs,
	})
}

type approverJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

func (c *Controller) DescribeDesktopSignIn(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, app.ErrDesktopSignInNotFound.Error())
	}
	v, err := c.service.DescribeDesktopSignIn(ctx.Context(), id, ctx.Request().Query("token"))
	if errors.Is(err, app.ErrDesktopSignInNotFound) {
		return respond.Error(ctx, contractshttp.StatusNotFound, err.Error())
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	_, signedIn, err := SessionMember(c.service, ctx)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	var approvedBy *approverJSON
	if v.ApprovedBy != nil {
		approvedBy = &approverJSON{ID: v.ApprovedBy.ID, Name: v.ApprovedBy.Name}
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"id":               v.SignIn.ID,
		"client_name":      v.SignIn.ClientName,
		"status":           v.Status,
		"expires_at":       v.SignIn.ExpiresAt,
		"approved_at":      v.SignIn.ApprovedAt,
		"approved_by":      approvedBy,
		"requires_sign_in": !signedIn,
		"can_approve":      signedIn && v.Status == domain.DesktopSignInPending,
	})
}

type desktopSignInTokenRequest struct {
	Token string `json:"token"`
}

// desktopSignInRefused answers an approve or cancel the sign-in's status
// refuses; false for any other error.
func desktopSignInRefused(ctx contractshttp.Context, err error) (contractshttp.Response, bool) {
	switch {
	case errors.Is(err, app.ErrDesktopSignInNotFound):
		return respond.Error(ctx, contractshttp.StatusNotFound, err.Error()), true
	case errors.Is(err, domain.ErrDesktopSignInApproved), errors.Is(err, domain.ErrDesktopSignInCancelled), errors.Is(err, domain.ErrDesktopSignInExpired):
		return respond.Error(ctx, contractshttp.StatusUnprocessableEntity, err.Error()), true
	}
	return nil, false
}

// ApproveDesktopSignIn needs a Session: a bearer secret must not approve
// another machine.
func (c *Controller) ApproveDesktopSignIn(ctx contractshttp.Context) contractshttp.Response {
	if ctx.Request().Header("Authorization") != "" {
		return respond.Error(ctx, contractshttp.StatusForbidden, "this needs a signed-in session")
	}
	m, signedIn, err := SessionMember(c.service, ctx)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	if !signedIn {
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "not signed in")
	}
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, app.ErrDesktopSignInNotFound.Error())
	}
	var req desktopSignInTokenRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	in, err := c.service.ApproveDesktopSignIn(ctx.Context(), id, req.Token, m.ID)
	if res, refused := desktopSignInRefused(ctx, err); refused {
		return res
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"status": c.service.DesktopSignInStatus(in), "desktop_id": in.DesktopID})
}

func (c *Controller) CancelDesktopSignIn(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, app.ErrDesktopSignInNotFound.Error())
	}
	var req desktopSignInTokenRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	in, err := c.service.CancelDesktopSignIn(ctx.Context(), id, req.Token)
	if res, refused := desktopSignInRefused(ctx, err); refused {
		return res
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"status": c.service.DesktopSignInStatus(in)})
}

type desktopJSON struct {
	ID         uint64     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	// Current is true for the Desktop making the request.
	Current bool `json:"current"`
}

// Desktops lists the signed-in Member's Desktops that are signed in.
func (c *Controller) Desktops(ctx contractshttp.Context) contractshttp.Response {
	p := principalOf(ctx)
	ds, err := c.service.Desktops(ctx.Context(), p.MemberID)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]desktopJSON, len(ds))
	for i, d := range ds {
		out[i] = desktopJSON{ID: d.ID, Name: d.Name, CreatedAt: d.CreatedAt.UTC(), LastSeenAt: d.LastSeenAt, Current: p.Desktop != nil && p.Desktop.ID == d.ID}
	}
	return ctx.Response().Success().Json(out)
}

// SignOutDesktop signs out one of the Member's own Desktops; another's is
// 404.
func (c *Controller) SignOutDesktop(ctx contractshttp.Context) contractshttp.Response {
	id, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, app.ErrDesktopNotFound.Error())
	}
	return c.signOutDesktop(ctx, id)
}

// SignOutCurrentDesktop is the Desktop app signing itself out.
func (c *Controller) SignOutCurrentDesktop(ctx contractshttp.Context) contractshttp.Response {
	p := principalOf(ctx)
	if p.Desktop == nil {
		return respond.Error(ctx, contractshttp.StatusForbidden, "this needs a desktop key")
	}
	return c.signOutDesktop(ctx, p.Desktop.ID)
}

func (c *Controller) signOutDesktop(ctx contractshttp.Context, id uint64) contractshttp.Response {
	err := c.service.SignOutDesktop(ctx.Context(), principalOf(ctx).MemberID, id)
	if errors.Is(err, app.ErrDesktopNotFound) {
		return respond.Error(ctx, contractshttp.StatusNotFound, err.Error())
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().NoContent()
}
