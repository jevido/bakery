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
	Redirect        string        `json:"redirect"`
	ResponseHeaders []headerJSON  `json:"response_headers"`
	BasicAuth       basicAuthJSON `json:"basic_auth"`
}

type headerJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// basicAuthJSON never carries the hash out. Password is only read: empty
// keeps the stored one.
type basicAuthJSON struct {
	Enabled     bool   `json:"enabled"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	PasswordSet bool   `json:"password_set"`
}

func settingsToJSON(s domain.RouteSettings) settingsJSON {
	out := settingsJSON{
		Redirect:        string(s.Redirect),
		ResponseHeaders: make([]headerJSON, len(s.ResponseHeaders)),
		BasicAuth:       basicAuthJSON{Enabled: s.BasicAuth.Enabled, Username: s.BasicAuth.Username, PasswordSet: s.BasicAuth.PasswordHash != ""},
	}
	for i, h := range s.ResponseHeaders {
		out.ResponseHeaders[i] = headerJSON(h)
	}
	return out
}

func (r settingsJSON) settings(applicationID uint64) domain.RouteSettings {
	s := domain.RouteSettings{
		ApplicationID: applicationID, Redirect: domain.Redirect(r.Redirect),
		ResponseHeaders: make([]domain.ResponseHeader, len(r.ResponseHeaders)),
		BasicAuth:       domain.BasicAuth{Enabled: r.BasicAuth.Enabled, Username: r.BasicAuth.Username},
	}
	for i, h := range r.ResponseHeaders {
		s.ResponseHeaders[i] = domain.ResponseHeader(h)
	}
	return s
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
	s, err := c.service.ChangeRouteSettings(ctx.Context(), req.settings(id), req.BasicAuth.Password)
	var fe *domain.FieldError
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"routing": settingsToJSON(s)})
}
