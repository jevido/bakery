// Package http is the services JSON API.
package http

import (
	"errors"
	"strconv"
	"strings"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/services/app"
	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

type Controller struct {
	service *app.Service
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

type componentJSON struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Public  bool     `json:"public"`
	Port    *int     `json:"port"`
	Domains []string `json:"domains"`
	URL     string   `json:"url,omitempty"`
	Status  string   `json:"status"`
	Detail  string   `json:"detail,omitempty"`
}

type variableJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Magic is the kind of Magic variable ("password", "user", …), "" for
	// one the Owner sets.
	Magic   string  `json:"magic"`
	Default *string `json:"default"`
	// Hidden says Value was left out for a viewer.
	Hidden bool `json:"hidden,omitempty"`
}

type serviceJSON struct {
	ID            uint64          `json:"id"`
	EnvironmentID uint64          `json:"environment_id"`
	ProjectID     uint64          `json:"project_id"`
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	TemplateKey   string          `json:"template,omitempty"`
	Status        string          `json:"status"`
	DesiredState  string          `json:"desired_state"`
	Busy          bool            `json:"busy"`
	LastError     string          `json:"last_error,omitempty"`
	Components    []componentJSON `json:"components"`
	// Only on a single Service: the list stays free of secrets.
	Compose   *string         `json:"compose,omitempty"`
	Variables *[]variableJSON `json:"variables,omitempty"`
}

func toJSON(v app.View, full bool) serviceJSON {
	out := serviceJSON{
		ID: v.ID, EnvironmentID: v.EnvironmentID, ProjectID: v.ProjectID,
		Name: v.Name, Slug: v.Slug, TemplateKey: v.TemplateKey,
		Status: string(v.Status), DesiredState: string(v.DesiredState), Busy: v.Busy, LastError: v.LastError,
		Components: make([]componentJSON, 0, len(v.Components)),
	}
	for _, c := range v.Components {
		cj := componentJSON{Name: c.Name, Image: c.Image, Public: c.Public, Domains: c.Domains, Status: string(c.Status), Detail: c.Detail}
		if cj.Domains == nil {
			cj.Domains = []string{}
		}
		if c.Public {
			port := c.Port
			cj.Port = &port
			cj.URL = "https://" + c.PrimaryDomain()
		}
		out.Components = append(out.Components, cj)
	}
	if full {
		compose := v.ComposeFile
		out.Compose = &compose
		vars := make([]variableJSON, 0, len(v.Variables))
		for _, va := range v.Variables {
			vj := variableJSON{Name: va.Name, Value: va.Value, Magic: string(va.Magic)}
			if va.HasDefault {
				def := va.Default
				vj.Default = &def
			}
			vars = append(vars, vj)
		}
		out.Variables = &vars
	}
	return out
}

func id(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func notFound(ctx contractshttp.Context) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
}

func fail(ctx contractshttp.Context, err error) contractshttp.Response {
	var fe *domain.FieldError
	var ce *domain.ComposeFileError
	switch {
	case errors.As(err, &ce):
		// Every line of the Compose file that is refused, so the dashboard
		// can show them all at once.
		lines := ce.Errors.Messages()
		return ctx.Response().Json(contractshttp.StatusUnprocessableEntity, contractshttp.Json{
			"message":        lines[0],
			"errors":         map[string]string{"compose": strings.Join(lines, "\n")},
			"compose_errors": lines,
		})
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.Is(err, app.ErrNotFound):
		return notFound(ctx)
	case errors.Is(err, app.ErrBusy):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	}
	return respond.ServerError(ctx, err)
}

func one(ctx contractshttp.Context, status int, v app.View, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out := toJSON(v, true)
	if !identity.CanSeeSecrets(ctx) && out.Variables != nil {
		for i := range *out.Variables {
			(*out.Variables)[i].Value, (*out.Variables)[i].Hidden = "", true
		}
	}
	return ctx.Response().Json(status, contractshttp.Json{"service": out})
}

// createRequest names a template or carries a Compose file.
type createRequest struct {
	Name     string `json:"name"`
	Compose  string `json:"compose"`
	Template string `json:"template"`
}

func (c *Controller) Create(ctx contractshttp.Context) contractshttp.Response {
	envID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req createRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	var v app.View
	var err error
	if req.Template != "" {
		v, err = c.service.CreateFromTemplate(ctx.Context(), envID, req.Template, req.Name)
	} else {
		v, err = c.service.Create(ctx.Context(), envID, app.Input{Name: req.Name, Compose: req.Compose})
	}
	return one(ctx, contractshttp.StatusCreated, v, err)
}

type templateJSON struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	DocsURL     string   `json:"docs_url"`
	Category    string   `json:"category"`
	Logo        string   `json:"logo"`
	Tags        []string `json:"tags"`
}

// Templates lists the catalog.
func (c *Controller) Templates(ctx contractshttp.Context) contractshttp.Response {
	list := c.service.Templates()
	out := make([]templateJSON, len(list))
	for i, t := range list {
		out[i] = templateJSON{Key: t.Key, Name: t.Name, Description: t.Description, DocsURL: t.DocsURL, Category: t.Category, Logo: t.Logo, Tags: t.Tags}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"templates": out})
}

func (c *Controller) ForProject(ctx contractshttp.Context) contractshttp.Response {
	pid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	list, err := c.service.ForProject(ctx.Context(), pid)
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]serviceJSON, len(list))
	for i, v := range list {
		out[i] = toJSON(v, false)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"services": out})
}

func (c *Controller) Show(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := c.service.Get(ctx.Context(), sid)
	return one(ctx, contractshttp.StatusOK, v, err)
}

type updateRequest struct {
	Name      *string             `json:"name"`
	Compose   *string             `json:"compose"`
	Domains   map[string][]string `json:"domains"`
	Variables map[string]string   `json:"variables"`
}

func (c *Controller) Update(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req updateRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	v, err := c.service.Update(ctx.Context(), sid, app.Change{Name: req.Name, Compose: req.Compose, Domains: req.Domains, Variables: req.Variables})
	return one(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) action(ctx contractshttp.Context, f func(*app.Service, contractshttp.Context, uint64) (app.View, error)) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := f(c.service, ctx, sid)
	return one(ctx, contractshttp.StatusAccepted, v, err)
}

func (c *Controller) Start(ctx contractshttp.Context) contractshttp.Response {
	return c.action(ctx, func(s *app.Service, ctx contractshttp.Context, id uint64) (app.View, error) {
		return s.Start(ctx.Context(), id)
	})
}

func (c *Controller) Stop(ctx contractshttp.Context) contractshttp.Response {
	return c.action(ctx, func(s *app.Service, ctx contractshttp.Context, id uint64) (app.View, error) {
		return s.Stop(ctx.Context(), id)
	})
}

func (c *Controller) Restart(ctx contractshttp.Context) contractshttp.Response {
	return c.action(ctx, func(s *app.Service, ctx contractshttp.Context, id uint64) (app.View, error) {
		return s.Restart(ctx.Context(), id)
	})
}

func (c *Controller) Redeploy(ctx contractshttp.Context) contractshttp.Response {
	return c.action(ctx, func(s *app.Service, ctx contractshttp.Context, id uint64) (app.View, error) {
		return s.Redeploy(ctx.Context(), id)
	})
}

func (c *Controller) Delete(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.Delete(ctx.Context(), sid); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
