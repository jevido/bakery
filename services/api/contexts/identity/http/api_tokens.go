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
	ID         uint64     `json:"id"`
	Name       string     `json:"name"`
	ReadOnly   bool       `json:"read_only"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func apiTokenToJSON(t domain.APIToken) apiTokenJSON {
	return apiTokenJSON{ID: t.ID, Name: t.Name, ReadOnly: t.ReadOnly, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt}
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
	Name     string `json:"name"`
	ReadOnly bool   `json:"read_only"`
}

// CreateAPIToken answers the token's value, the only time it is shown.
func (c *Controller) CreateAPIToken(ctx contractshttp.Context) contractshttp.Response {
	var req apiTokenRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	id, _ := MemberID(ctx)
	t, value, err := c.service.CreateAPIToken(ctx.Context(), id, req.Name, req.ReadOnly)
	switch {
	case errors.Is(err, domain.ErrInvalidTokenName), errors.Is(err, app.ErrTokenNameTaken):
		return respond.Invalid(ctx, "name", err.Error())
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
