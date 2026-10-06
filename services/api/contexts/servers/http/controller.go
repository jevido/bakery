// Package http is the servers JSON API.
package http

import (
	"errors"
	"strconv"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/servers/app"
	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

type Controller struct {
	service *app.Service
	// Guild is the Current guild of the request.
	Guild func(ctx contractshttp.Context) uint64
	// InstanceAdmin reports whether the request comes from the Instance
	// admin, the only one who changes or cleans up the Local server.
	InstanceAdmin func(ctx contractshttp.Context) bool
	// SeesContainer reports whether the request may see a Container on the
	// Local server, by its owner.
	SeesContainer func(ctx contractshttp.Context, owner, ownerID string) (bool, error)
}

func NewController(service *app.Service) *Controller {
	return &Controller{service: service}
}

type checkJSON struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

type validationJSON struct {
	Checks    []checkJSON `json:"checks"`
	CheckedAt *time.Time  `json:"checked_at"`
}

type cleanupJSON struct {
	At             *time.Time `json:"at"`
	ReclaimedBytes int64      `json:"reclaimed_bytes"`
}

type serverJSON struct {
	ID          uint64         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Kind        string         `json:"kind"`
	Host        string         `json:"host"`
	Port        int            `json:"port"`
	User        string         `json:"user"`
	Status      string         `json:"status"`
	Validation  validationJSON `json:"validation"`
	LastCleanup cleanupJSON    `json:"last_cleanup"`
	CreatedAt   time.Time      `json:"created_at"`
	// Only on a single Server. The private key is never sent.
	PublicKey          string `json:"public_key,omitempty"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`
}

func cleanupToJSON(c domain.Cleanup) cleanupJSON {
	out := cleanupJSON{ReclaimedBytes: c.Reclaimed}
	if !c.At.IsZero() {
		at := c.At
		out.At = &at
	}
	return out
}

func toJSON(s domain.Server, full bool) serverJSON {
	out := serverJSON{
		ID: s.ID, Name: s.Name, Description: s.Description, Kind: string(s.Kind), Host: s.Host, Port: s.Port, User: s.User,
		Status: string(s.Status), Validation: validationJSON{Checks: []checkJSON{}},
		LastCleanup: cleanupToJSON(s.LastCleanup), CreatedAt: s.CreatedAt,
	}
	for _, c := range s.Validation.Checks {
		out.Validation.Checks = append(out.Validation.Checks, checkJSON(c))
	}
	if !s.Validation.CheckedAt.IsZero() {
		at := s.Validation.CheckedAt
		out.Validation.CheckedAt = &at
	}
	if full {
		out.PublicKey = s.Key.Public
		out.HostKeyFingerprint = fingerprint(s.HostKey)
	}
	return out
}

func fingerprint(hostKey string) string {
	if strings.TrimSpace(hostKey) == "" {
		return ""
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
	if err != nil {
		return ""
	}
	return k.Type() + " " + ssh.FingerprintSHA256(k)
}

type serverRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
}

func (r serverRequest) input() domain.Input {
	return domain.Input{Name: r.Name, Description: r.Description, Host: r.Host, Port: r.Port, User: r.User}
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
	var unreachable *app.ErrUnreachable
	switch {
	case errors.As(err, &fe):
		return respond.Invalid(ctx, fe.Field, fe.Message)
	case errors.Is(err, app.ErrNotFound):
		return notFound(ctx)
	case errors.Is(err, domain.ErrLocalServer), errors.Is(err, app.ErrInUse):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	case errors.As(err, &unreachable):
		return respond.Error(ctx, contractshttp.StatusConflict, err.Error())
	}
	return respond.ServerError(ctx, err)
}

func one(ctx contractshttp.Context, status int, s domain.Server, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"server": toJSON(s, true)})
}

func (c *Controller) List(ctx contractshttp.Context) contractshttp.Response {
	list, err := c.service.List(ctx.Context(), c.Guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]serverJSON, len(list))
	for i, s := range list {
		out[i] = toJSON(s, false)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"servers": out})
}

func (c *Controller) Create(ctx contractshttp.Context) contractshttp.Response {
	var req serverRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	s, err := c.service.Add(ctx.Context(), c.Guild(ctx), req.input())
	return one(ctx, contractshttp.StatusCreated, s, err)
}

func (c *Controller) Show(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	s, err := c.service.Get(ctx.Context(), sid)
	return one(ctx, contractshttp.StatusOK, s, err)
}

// localRefused answers 403 when the Server is the Local server and the
// request is not the Instance admin's; ok is false when it answered.
func (c *Controller) localRefused(ctx contractshttp.Context, sid uint64) (contractshttp.Response, bool) {
	s, err := c.service.Get(ctx.Context(), sid)
	if err != nil {
		return fail(ctx, err), false
	}
	if s.Kind == domain.Local && !c.InstanceAdmin(ctx) {
		return respond.Error(ctx, contractshttp.StatusForbidden, "the local server is the instance's"), false
	}
	return nil, true
}

func (c *Controller) Update(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if r, ok := c.localRefused(ctx, sid); !ok {
		return r
	}
	var req serverRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	s, err := c.service.Edit(ctx.Context(), sid, req.input())
	return one(ctx, contractshttp.StatusOK, s, err)
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

func (c *Controller) Validate(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	s, err := c.service.Validate(ctx.Context(), sid)
	return one(ctx, contractshttp.StatusOK, s, err)
}

func (c *Controller) ForgetHostKey(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	s, err := c.service.ForgetHostKey(ctx.Context(), sid)
	return one(ctx, contractshttp.StatusOK, s, err)
}

type serverMetricsJSON struct {
	CPUs            int       `json:"cpus"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsed      int64     `json:"memory_used_bytes"`
	MemoryTotal     int64     `json:"memory_total_bytes"`
	DiskUsed        int64     `json:"disk_used_bytes"`
	DiskTotal       int64     `json:"disk_total_bytes"`
	ImagesBytes     int64     `json:"images_bytes"`
	ContainersBytes int64     `json:"containers_bytes"`
	VolumesBytes    int64     `json:"volumes_bytes"`
	ReadAt          time.Time `json:"read_at"`
}

type containerMetricsJSON struct {
	Name        string  `json:"name"`
	Owner       string  `json:"owner"`
	OwnerID     string  `json:"owner_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryUsed  int64   `json:"memory_used_bytes"`
	MemoryLimit int64   `json:"memory_limit_bytes"`
}

func (c *Controller) Metrics(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	m, err := c.service.Metrics(ctx.Context(), sid)
	if err != nil {
		return fail(ctx, err)
	}
	srv, err := c.service.Get(ctx.Context(), sid)
	if err != nil {
		return fail(ctx, err)
	}
	s := m.Server
	containers := []containerMetricsJSON{}
	for _, ct := range m.Containers {
		// Every Guild deploys to the Local server: show only its own there.
		if srv.Kind == domain.Local {
			seen, err := c.SeesContainer(ctx, ct.Owner, ct.OwnerID)
			if err != nil {
				return fail(ctx, err)
			}
			if !seen {
				continue
			}
		}
		containers = append(containers, containerMetricsJSON{
			Name: ct.Name, Owner: ct.Owner, OwnerID: ct.OwnerID,
			CPUPercent: ct.CPUPercent, MemoryUsed: ct.MemUsed, MemoryLimit: ct.MemLimit,
		})
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"server": serverMetricsJSON{
			CPUs: s.CPUs, CPUPercent: s.CPUPercent, MemoryUsed: s.MemUsed, MemoryTotal: s.MemTotal,
			DiskUsed: s.DiskUsed, DiskTotal: s.DiskTotal,
			ImagesBytes: s.Images, ContainersBytes: s.Containers, VolumesBytes: s.Volumes, ReadAt: s.ReadAt,
		},
		"containers": containers,
	})
}

type detailsJSON struct {
	OS            string     `json:"os"`
	Arch          string     `json:"arch"`
	Kernel        string     `json:"kernel"`
	CPUs          int        `json:"cpus"`
	MemoryBytes   int64      `json:"memory_bytes"`
	PodmanVersion string     `json:"podman_version"`
	UpSince       *time.Time `json:"up_since"`
}

func (c *Controller) Details(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	d, err := c.service.Details(ctx.Context(), sid)
	if err != nil {
		return fail(ctx, err)
	}
	out := detailsJSON{
		OS: d.OS, Arch: d.Arch, Kernel: d.Kernel, CPUs: d.CPUs, MemoryBytes: d.Memory, PodmanVersion: d.PodmanVersion,
	}
	if !d.UpSince.IsZero() {
		at := d.UpSince.UTC()
		out.UpSince = &at
	}
	return ctx.Response().Success().Json(contractshttp.Json{"details": out})
}

func (c *Controller) CleanUp(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if r, ok := c.localRefused(ctx, sid); !ok {
		return r
	}
	cl, err := c.service.CleanUp(ctx.Context(), sid)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"cleanup": cleanupToJSON(cl)})
}
