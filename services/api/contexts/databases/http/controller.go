// Package http is the databases JSON API.
package http

import (
	"errors"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/databases/app"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

type Controller struct {
	service *app.Service
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

// limitsJSON is null for unlimited, like an Application's.
type limitsJSON struct {
	MemoryMB *int     `json:"memory_mb"`
	CPUs     *float64 `json:"cpus"`
}

func limitsToJSON(l domain.ResourceLimits) limitsJSON {
	var out limitsJSON
	if l.MemoryMB != 0 {
		out.MemoryMB = &l.MemoryMB
	}
	if l.CPUs != 0 {
		out.CPUs = &l.CPUs
	}
	return out
}

func (r limitsJSON) limits() domain.ResourceLimits {
	var l domain.ResourceLimits
	if r.MemoryMB != nil {
		l.MemoryMB = *r.MemoryMB
	}
	if r.CPUs != nil {
		l.CPUs = *r.CPUs
	}
	return l
}

type credentialsJSON struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	RootPassword string `json:"root_password,omitempty"`
	DatabaseName string `json:"database_name"`
}

type databaseJSON struct {
	ID             uint64     `json:"id"`
	EnvironmentID  uint64     `json:"environment_id"`
	ProjectID      uint64     `json:"project_id"`
	Name           string     `json:"name"`
	Slug           string     `json:"slug"`
	Engine         string     `json:"engine"`
	Version        string     `json:"version"`
	Status         string     `json:"status"`
	DesiredState   string     `json:"desired_state"`
	Error          string     `json:"error,omitempty"`
	PublicPort     *int       `json:"public_port"`
	ResourceLimits limitsJSON `json:"resource_limits"`
	// Only on a single Database: the list stays free of secrets.
	Credentials *credentialsJSON `json:"credentials,omitempty"`
	InternalURL string           `json:"internal_url,omitempty"`
	PublicURL   *string          `json:"public_url,omitempty"`

	BackupsSupported bool         `json:"backups_supported"`
	BackupSchedule   scheduleJSON `json:"backup_schedule"`
	NextBackupAt     *time.Time   `json:"next_backup_at"`
	Restoring        bool         `json:"restoring"`
	LastRestore      *restoreJSON `json:"last_restore"`
}

func toJSON(v app.View, full bool) databaseJSON {
	out := databaseJSON{
		ID: v.ID, EnvironmentID: v.EnvironmentID, ProjectID: v.ProjectID,
		Name: v.Name, Slug: v.Slug, Engine: string(v.Engine), Version: v.Version,
		Status: string(v.Status), DesiredState: string(v.DesiredState), Error: v.Error,
		ResourceLimits:   limitsToJSON(v.ResourceLimits),
		BackupsSupported: v.Engine.Spec().Backups, BackupSchedule: scheduleToJSON(v.BackupSchedule),
		Restoring: v.Restoring,
	}
	if !v.NextBackupAt.IsZero() {
		next := v.NextBackupAt
		out.NextBackupAt = &next
	}
	if r := v.LastRestore; r != nil {
		out.LastRestore = &restoreJSON{BackupID: r.BackupID, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Error: r.Error}
	}
	if v.PublicPort != 0 {
		port := v.PublicPort
		out.PublicPort = &port
	}
	if full {
		c := credentialsJSON(v.Credentials)
		out.Credentials = &c
		out.InternalURL = v.InternalURL
		public := v.PublicURL
		out.PublicURL = &public
		if public == "" {
			out.PublicURL = nil
		}
	}
	return out
}

// databaseRequest is the whole Database as the Owner sets it; engine is
// only read on creation, public_port null (or 0) is none.
type databaseRequest struct {
	Name           string     `json:"name"`
	Engine         string     `json:"engine"`
	Version        string     `json:"version"`
	PublicPort     *int       `json:"public_port"`
	ResourceLimits limitsJSON `json:"resource_limits"`
}

func (r databaseRequest) input() domain.Input {
	in := domain.Input{Name: r.Name, Engine: domain.Engine(r.Engine), Version: r.Version, ResourceLimits: r.ResourceLimits.limits()}
	if r.PublicPort != nil {
		in.PublicPort = *r.PublicPort
	}
	return in
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
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.Is(err, app.ErrNotFound):
		return notFound(ctx)
	}
	return respond.ServerError(ctx, err)
}

func one(ctx contractshttp.Context, status int, v app.View, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"database": toJSON(v, true)})
}

func (c *Controller) Create(ctx contractshttp.Context) contractshttp.Response {
	envID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req databaseRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	v, err := c.service.Create(ctx.Context(), envID, req.input())
	return one(ctx, contractshttp.StatusCreated, v, err)
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
	out := make([]databaseJSON, len(list))
	for i, v := range list {
		out[i] = toJSON(v, false)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"databases": out})
}

func (c *Controller) Show(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := c.service.Get(ctx.Context(), dbID)
	return one(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) Update(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req databaseRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	v, err := c.service.Update(ctx.Context(), dbID, req.input())
	return one(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) Start(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := c.service.Start(ctx.Context(), dbID)
	return one(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) Stop(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := c.service.Stop(ctx.Context(), dbID)
	return one(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) Restart(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := c.service.Restart(ctx.Context(), dbID)
	return one(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) Delete(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.Delete(ctx.Context(), dbID); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
