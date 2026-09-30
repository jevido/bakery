// Package http is routing's JSON API: an Application's Route settings.
package http

import (
	"context"
	"errors"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/routing/app"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

// ApplicationExists is projects' published ApplicationExists.
type ApplicationExists func(ctx context.Context, id uint64) (bool, error)

type Controller struct {
	service *app.Service
	exists  ApplicationExists
}

func NewController(service *app.Service, exists ApplicationExists) *Controller {
	return &Controller{service: service, exists: exists}
}

type settingsJSON struct {
	WwwRedirect string `json:"www_redirect"`
}

func settingsToJSON(s domain.RouteSettings) settingsJSON {
	return settingsJSON{WwwRedirect: string(s.WwwRedirect)}
}

// application reads the {id} route parameter of an existing Application.
func (c *Controller) application(ctx contractshttp.Context) (uint64, contractshttp.Response) {
	id, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	if err != nil {
		return 0, respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	ok, err := c.exists(ctx.Context(), id)
	if err != nil {
		return 0, respond.ServerError(ctx, err)
	}
	if !ok {
		return 0, respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	return id, nil
}

func (c *Controller) ShowSettings(ctx contractshttp.Context) contractshttp.Response {
	id, fail := c.application(ctx)
	if fail != nil {
		return fail
	}
	s, err := c.service.RouteSettings(ctx.Context(), id)
	if err != nil {
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"routing": settingsToJSON(s)})
}

// ReplaceSettings stores the whole set and Applies it at once.
func (c *Controller) ReplaceSettings(ctx contractshttp.Context) contractshttp.Response {
	id, fail := c.application(ctx)
	if fail != nil {
		return fail
	}
	var req settingsJSON
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	s, err := c.service.ChangeRouteSettings(ctx.Context(), domain.RouteSettings{ApplicationID: id, WwwRedirect: domain.WwwRedirect(req.WwwRedirect)})
	var fe *domain.FieldError
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"routing": settingsToJSON(s)})
}
