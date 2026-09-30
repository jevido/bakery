// Package http exposes deployments over HTTP.
package http

import (
	"errors"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type Controller struct {
	service *app.Service
	// isNotFound recognises projects' "no such application".
	isNotFound func(error) bool
}

func NewController(service *app.Service, isNotFound func(error) bool) *Controller {
	return &Controller{service: service, isNotFound: isNotFound}
}

// LocalServer returns the Local server's id, shown where a Deployment's
// Server is 0; nil or failing shows 0.
var LocalServer func() uint64

type deploymentJSON struct {
	ID            uint64     `json:"id"`
	ApplicationID uint64     `json:"application_id"`
	ServerID      uint64     `json:"server_id"`
	Status        string     `json:"status"`
	Active        bool       `json:"active"`
	Trigger       string     `json:"trigger"`
	Branch        string     `json:"branch"`
	CommitSHA     string     `json:"commit_sha"`
	CommitMessage string     `json:"commit_message"`
	CommitAuthor  string     `json:"commit_author"`
	SourceImage   string     `json:"source_image"`
	Image         string     `json:"image"`
	Container     string     `json:"container"`
	RollbackOf    *uint64    `json:"rollback_of"`
	Error         string     `json:"error"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
}

func ToJSON(d domain.Deployment) deploymentJSON {
	server := d.ServerID
	if server == 0 && LocalServer != nil {
		server = LocalServer()
	}
	return deploymentJSON{
		ID: d.ID, ApplicationID: d.ApplicationID, ServerID: server, Status: string(d.Status), Active: d.Status.Active(),
		Trigger: string(d.Trigger), Branch: d.Branch, CommitSHA: d.CommitSHA,
		CommitMessage: d.CommitMessage, CommitAuthor: d.CommitAuthor, SourceImage: d.SourceImage, Image: d.Image, Container: d.Container, RollbackOf: d.RollbackOf, Error: d.Error,
		CreatedAt: d.CreatedAt, StartedAt: d.StartedAt, FinishedAt: d.FinishedAt,
	}
}

func (c *Controller) fail(ctx contractshttp.Context, err error) contractshttp.Response {
	switch {
	case errors.Is(err, app.ErrNotFound), c.isNotFound(err):
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrAlreadyQueued), errors.Is(err, app.ErrNotCancellable),
		errors.Is(err, domain.ErrNotRollbackTarget), errors.Is(err, app.ErrImageGone):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	}
	return respond.ServerError(ctx, err)
}

// RouteID reads the {id} route parameter.
func RouteID(ctx contractshttp.Context) (uint64, bool) {
	v, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	return v, err == nil
}

func (c *Controller) Deploy(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Deploy(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"deployment": ToJSON(d)})
}

func (c *Controller) List(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	ds, err := c.service.Deployments(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	out := make([]deploymentJSON, len(ds))
	for i, d := range ds {
		out[i] = ToJSON(d)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"deployments": out})
}

func (c *Controller) Show(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Deployment(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"deployment": ToJSON(d)})
}

func (c *Controller) Rollback(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Rollback(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusCreated, contractshttp.Json{"deployment": ToJSON(d)})
}

// Cancel answers 200 with a Deployment cancelled at once (it was queued),
// or 202 with a running one that the Worker is stopping.
func (c *Controller) Cancel(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	d, err := c.service.Cancel(ctx.Context(), id)
	if err != nil {
		return c.fail(ctx, err)
	}
	status := contractshttp.StatusOK
	if d.Status.Active() {
		status = contractshttp.StatusAccepted
	}
	return ctx.Response().Json(status, contractshttp.Json{"deployment": ToJSON(d)})
}

type knownHostJSON struct {
	ID           uint64    `json:"id"`
	Host         string    `json:"host"`
	Fingerprints []string  `json:"fingerprints"`
	CreatedAt    time.Time `json:"created_at"`
}

func (c *Controller) KnownHosts(ctx contractshttp.Context) contractshttp.Response {
	hosts, err := c.service.KnownHosts(ctx.Context())
	if err != nil {
		return c.fail(ctx, err)
	}
	out := make([]knownHostJSON, len(hosts))
	for i, h := range hosts {
		out[i] = knownHostJSON{ID: h.ID, Host: h.Host, Fingerprints: h.Fingerprints, CreatedAt: h.CreatedAt}
		if out[i].Fingerprints == nil {
			out[i].Fingerprints = []string{}
		}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"known_hosts": out})
}

func (c *Controller) ForgetKnownHost(ctx contractshttp.Context) contractshttp.Response {
	id, ok := RouteID(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusNotFound, "not found")
	}
	if err := c.service.ForgetKnownHost(ctx.Context(), id); err != nil {
		return c.fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
