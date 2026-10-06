package http

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/databases/app"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// conflict maps the errors that mean "not now" or "not this one" to 409,
// and a Database type without backups to 422.
func conflict(ctx contractshttp.Context, err error) (contractshttp.Response, bool) {
	for _, e := range []error{app.ErrBusy, app.ErrNotRunning, app.ErrBackupRunning, app.ErrNotRestorable, app.ErrS3StorageInUse} {
		if errors.Is(err, e) {
			return respond.Error(ctx, contractshttp.StatusConflict, e.Error()), true
		}
	}
	if errors.Is(err, domain.ErrNoBackups) {
		return respond.Error(ctx, contractshttp.StatusUnprocessableEntity, err.Error()), true
	}
	return nil, false
}

func failBackup(ctx contractshttp.Context, err error) contractshttp.Response {
	if res, ok := conflict(ctx, err); ok {
		return res
	}
	return fail(ctx, err)
}

type scheduledBackupJSON struct {
	ID           uint64     `json:"id"`
	DatabaseID   uint64     `json:"database_id"`
	Enabled      bool       `json:"enabled"`
	Cron         string     `json:"cron"`
	Retention    int        `json:"retention"`
	S3StorageID  *uint64    `json:"s3_storage_id"`
	NextBackupAt *time.Time `json:"next_backup_at,omitempty"`
}

func scheduledBackupToJSON(s domain.ScheduledBackup) scheduledBackupJSON {
	out := scheduledBackupJSON{ID: s.ID, DatabaseID: s.DatabaseID, Enabled: s.Enabled, Cron: s.Cron, Retention: s.Retention}
	if s.S3StorageID != 0 {
		id := s.S3StorageID
		out.S3StorageID = &id
	}
	return out
}

func scheduledBackupViewToJSON(v app.ScheduledBackupView) scheduledBackupJSON {
	out := scheduledBackupToJSON(v.ScheduledBackup)
	if !v.NextBackupAt.IsZero() {
		next := v.NextBackupAt
		out.NextBackupAt = &next
	}
	return out
}

// scheduledBackupRequest is a Scheduled backup as the Owner sets it.
type scheduledBackupRequest struct {
	Enabled     bool    `json:"enabled"`
	Cron        string  `json:"cron"`
	Retention   int     `json:"retention"`
	S3StorageID *uint64 `json:"s3_storage_id"`
}

func (r scheduledBackupRequest) input() domain.ScheduledBackupInput {
	in := domain.ScheduledBackupInput{Enabled: r.Enabled, Cron: r.Cron, Retention: r.Retention}
	if r.S3StorageID != nil {
		in.S3StorageID = *r.S3StorageID
	}
	return in
}

type restoreJSON struct {
	BackupExecutionID uint64    `json:"backup_execution_id"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	Error             string    `json:"error,omitempty"`
}

type backupExecutionJSON struct {
	ID                uint64     `json:"id"`
	DatabaseID        uint64     `json:"database_id"`
	ScheduledBackupID uint64     `json:"scheduled_backup_id"`
	Status            string     `json:"status"`
	Trigger           string     `json:"trigger"`
	FileName          string     `json:"file_name"`
	SizeBytes         int64      `json:"size_bytes"`
	Local             bool       `json:"local"`
	S3                bool       `json:"s3"`
	Error             string     `json:"error,omitempty"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
}

func executionToJSON(b domain.BackupExecution) backupExecutionJSON {
	out := backupExecutionJSON{
		ID: b.ID, DatabaseID: b.DatabaseID, ScheduledBackupID: b.ScheduledBackupID, Status: string(b.Status), Trigger: string(b.Trigger),
		FileName: b.FileName, SizeBytes: b.SizeBytes, Local: b.Local, S3: b.S3, Error: b.Error,
		StartedAt: b.StartedAt,
	}
	if !b.FinishedAt.IsZero() {
		f := b.FinishedAt
		out.FinishedAt = &f
	}
	return out
}

func executionsJSON(ctx contractshttp.Context, list []domain.BackupExecution, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]backupExecutionJSON, len(list))
	for i, b := range list {
		out[i] = executionToJSON(b)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"backup_executions": out})
}

func started(ctx contractshttp.Context, b domain.BackupExecution, err error) contractshttp.Response {
	if err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusAccepted, contractshttp.Json{"backup_execution": executionToJSON(b)})
}

func oneScheduledBackup(ctx contractshttp.Context, status int, v app.ScheduledBackupView, err error) contractshttp.Response {
	if errors.Is(err, domain.ErrNoBackups) {
		return respond.Invalid(ctx, "scheduled_backup.enabled", err.Error())
	}
	if err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"scheduled_backup": scheduledBackupViewToJSON(v)})
}

func (c *Controller) ScheduledBackups(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	list, err := c.service.ScheduledBackups(ctx.Context(), dbID)
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]scheduledBackupJSON, len(list))
	for i, v := range list {
		out[i] = scheduledBackupViewToJSON(v)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"scheduled_backups": out})
}

func (c *Controller) CreateScheduledBackup(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req scheduledBackupRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	v, err := c.service.CreateScheduledBackup(ctx.Context(), dbID, req.input())
	return oneScheduledBackup(ctx, contractshttp.StatusCreated, v, err)
}

func (c *Controller) ShowScheduledBackup(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	v, err := c.service.ScheduledBackup(ctx.Context(), sID)
	return oneScheduledBackup(ctx, contractshttp.StatusOK, v, err)
}

func (c *Controller) UpdateScheduledBackup(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req scheduledBackupRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	v, err := c.service.UpdateScheduledBackup(ctx.Context(), sID, req.input())
	return oneScheduledBackup(ctx, contractshttp.StatusOK, v, err)
}

// DeleteScheduledBackup removes the Scheduled backup with its Backup
// executions: their files unless delete_local=false, their S3 objects only
// with delete_s3=true (Coolify's delete_s3, false by default).
func (c *Controller) DeleteScheduledBackup(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	copies := app.Copies{Local: ctx.Request().QueryBool("delete_local", true), S3: ctx.Request().QueryBool("delete_s3", false)}
	if err := c.service.DeleteScheduledBackup(ctx.Context(), sID, copies); err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().NoContent()
}

// BackupExecutions lists every Backup execution of the Database.
func (c *Controller) BackupExecutions(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	list, err := c.service.BackupExecutions(ctx.Context(), dbID)
	return executionsJSON(ctx, list, err)
}

func (c *Controller) ScheduledBackupExecutions(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	list, err := c.service.ScheduledBackupExecutions(ctx.Context(), sID)
	return executionsJSON(ctx, list, err)
}

// BackUp starts a Backup execution of the Database's oldest Scheduled
// backup now; it runs in the background.
func (c *Controller) BackUp(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	b, err := c.service.BackUpDatabase(ctx.Context(), dbID, domain.TriggerManual)
	return started(ctx, b, err)
}

// BackUpScheduledBackup is Back up now on one Scheduled backup.
func (c *Controller) BackUpScheduledBackup(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	b, err := c.service.BackUp(ctx.Context(), sID, domain.TriggerManual)
	return started(ctx, b, err)
}

// Restore starts restoring the Backup execution into its Database.
func (c *Controller) Restore(ctx contractshttp.Context) contractshttp.Response {
	bID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.Restore(ctx.Context(), bID); err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().Json(contractshttp.StatusAccepted, contractshttp.Json{"restoring": true})
}

// DeleteBackupExecution removes a Backup execution with its file and, unless
// delete_s3=false, its S3 object (the dashboard always says which).
func (c *Controller) DeleteBackupExecution(ctx contractshttp.Context) contractshttp.Response {
	bID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteBackupExecution(ctx.Context(), bID, ctx.Request().QueryBool("delete_s3", true)); err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().NoContent()
}

// Download streams the Backup execution file, from local disk or its S3 storage.
func (c *StreamController) Download(ctx contractshttp.Context) contractshttp.Response {
	bID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	reqCtx := ctx.Request().Origin().Context()
	r, size, name, err := c.service.OpenBackupExecution(reqCtx, bID)
	if err != nil {
		return failBackup(ctx, err)
	}
	defer r.Close()
	w := ctx.Response().Writer()
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	if size > 0 {
		h.Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.WriteHeader(contractshttp.StatusOK)
	_, _ = io.Copy(w, r)
	return nil
}

type s3StorageJSON struct {
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	AccessKey    string `json:"access_key"`
	HasSecretKey bool   `json:"has_secret_key"`
}

func s3ToJSON(st domain.S3Storage) s3StorageJSON {
	return s3StorageJSON{
		ID: st.ID, Name: st.Name, Endpoint: st.Endpoint, Region: st.Region, Bucket: st.Bucket,
		Prefix: st.Prefix, AccessKey: st.AccessKey, HasSecretKey: st.SecretKey != "",
	}
}

// s3Request is an S3 storage as typed; the secret key is write-only, and
// empty on an update keeps it. ID is only read by the connection test.
type s3Request struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func (r s3Request) input() domain.S3Input {
	return domain.S3Input{Name: r.Name, Endpoint: r.Endpoint, Region: r.Region, Bucket: r.Bucket, Prefix: r.Prefix, AccessKey: r.AccessKey, SecretKey: r.SecretKey}
}

func oneS3(ctx contractshttp.Context, status int, st domain.S3Storage, err error) contractshttp.Response {
	if err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"s3_storage": s3ToJSON(st)})
}

func (c *Controller) S3Storages(ctx contractshttp.Context) contractshttp.Response {
	list, err := c.service.S3Storages(ctx.Context(), c.guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]s3StorageJSON, len(list))
	for i, st := range list {
		out[i] = s3ToJSON(st)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"s3_storages": out})
}

func (c *Controller) CreateS3Storage(ctx contractshttp.Context) contractshttp.Response {
	var req s3Request
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	st, err := c.service.CreateS3Storage(ctx.Context(), c.guild(ctx), req.input())
	return oneS3(ctx, contractshttp.StatusCreated, st, err)
}

func (c *Controller) UpdateS3Storage(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req s3Request
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	st, err := c.service.UpdateS3Storage(ctx.Context(), sID, req.input())
	return oneS3(ctx, contractshttp.StatusOK, st, err)
}

func (c *Controller) DeleteS3Storage(ctx contractshttp.Context) contractshttp.Response {
	sID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteS3Storage(ctx.Context(), sID); err != nil {
		return failBackup(ctx, err)
	}
	return ctx.Response().NoContent()
}

// CheckS3Storage tests a connection with the typed settings (with id, an
// empty secret key is the stored one): {"ok": true} or {"ok": false,
// "message": what the storage answered}.
func (c *Controller) CheckS3Storage(ctx contractshttp.Context) contractshttp.Response {
	var req s3Request
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	connErr, err := c.service.CheckS3Storage(ctx.Context(), c.guild(ctx), req.ID, req.input())
	if err != nil {
		return fail(ctx, err)
	}
	if connErr != nil {
		return ctx.Response().Success().Json(contractshttp.Json{"ok": false, "message": connErr.Error()})
	}
	return ctx.Response().Success().Json(contractshttp.Json{"ok": true})
}
