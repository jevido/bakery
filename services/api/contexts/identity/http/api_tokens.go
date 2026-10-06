package http

import (
	"errors"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type apiTokenJSON struct {
	ID          uint64              `json:"id"`
	Name        string              `json:"name"`
	Permissions []domain.Permission `json:"permissions"`
	// ReadOnly is true when the Permissions are exactly read, for scripts
	// written before Permissions.
	ReadOnly   bool       `json:"read_only"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

func apiTokenToJSON(t domain.APIToken) apiTokenJSON {
	permissions := t.Permissions
	if permissions == nil {
		permissions = []domain.Permission{}
	}
	return apiTokenJSON{ID: t.ID, Name: t.Name, Permissions: permissions, ReadOnly: t.ReadOnly(), CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt, ExpiresAt: t.ExpiresAt}
}

func (c *Controller) APITokens(ctx contractshttp.Context) contractshttp.Response {
	id, _ := MemberID(ctx)
	tokens, err := c.service.APITokensOf(ctx.Context(), id)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	out := make([]apiTokenJSON, len(tokens))
	for i, t := range tokens {
		out[i] = apiTokenToJSON(t)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"api_tokens": out})
}

type apiTokenRequest struct {
	Name string `json:"name"`
	// Permissions absent means the request predates them: ReadOnly then
	// picks read, or else the most the Role may grant.
	Permissions *[]domain.Permission `json:"permissions"`
	ReadOnly    bool                 `json:"read_only"`
	// ExpiresInDays absent or null is Never.
	ExpiresInDays *int `json:"expires_in_days"`
}

// grantable is every Permission role may put on a token.
func grantable(role domain.Role) []domain.Permission {
	var out []domain.Permission
	for _, p := range domain.Permissions {
		if role.MayGrant(p) {
			out = append(out, p)
		}
	}
	return out
}

// CreateAPIToken answers the token's value, the only time it is shown.
func (c *Controller) CreateAPIToken(ctx contractshttp.Context) contractshttp.Response {
	var req apiTokenRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	var permissions []domain.Permission
	switch {
	case req.Permissions != nil:
		permissions = *req.Permissions
	case req.ReadOnly:
		permissions = []domain.Permission{domain.PermissionRead}
	case RoleOf(ctx).IsAdmin():
		permissions = []domain.Permission{domain.PermissionRoot}
	default:
		// root capped to the Role, so an old script's token does what it did.
		permissions = grantable(RoleOf(ctx))
	}
	t, value, err := c.service.CreateAPIToken(ctx.Context(), id, req.Name, permissions, req.ExpiresInDays)
	switch {
	case errors.Is(err, domain.ErrInvalidTokenName), errors.Is(err, app.ErrTokenNameTaken):
		return respond.Invalid(ctx, "name", err.Error())
	case errors.Is(err, domain.ErrUnknownPermission), errors.Is(err, domain.ErrRoleCannotGrant):
		return respond.Invalid(ctx, "permissions", err.Error())
	case errors.Is(err, app.ErrInvalidExpiry), errors.Is(err, domain.ErrTokenExpiryPassed):
		return respond.Invalid(ctx, "expires_in_days", err.Error())
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"api_token": apiTokenToJSON(t), "token": value})
}

func (c *Controller) RevokeAPIToken(ctx contractshttp.Context) contractshttp.Response {
	tokenID, ok := routeID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, app.ErrTokenNotFound.Error())
	}
	id, _ := MemberID(ctx)
	err := c.service.RevokeAPIToken(ctx.Context(), id, tokenID)
	if errors.Is(err, app.ErrTokenNotFound) {
		return respond.Error(ctx, contractshttp.StatusNotFound, err.Error())
	}
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().NoContent()
}

// APITokenPermissions answers which Permissions the signed-in Member may put
// on a token.
func (c *Controller) APITokenPermissions(ctx contractshttp.Context) contractshttp.Response {
	role := RoleOf(ctx)
	out := make([]contractshttp.Json, len(domain.Permissions))
	for i, p := range domain.Permissions {
		out[i] = contractshttp.Json{"name": p, "allowed": role.MayGrant(p)}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"permissions": out})
}
