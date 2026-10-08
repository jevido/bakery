package http

import (
	"context"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// documentJSON is an Issue document at its newest Revision.
type documentJSON struct {
	ID                   uint64    `json:"id"`
	Key                  string    `json:"key"`
	Title                string    `json:"title"`
	Body                 string    `json:"body"`
	Format               string    `json:"format"`
	LatestRevisionID     uint64    `json:"latest_revision_id"`
	LatestRevisionNumber int       `json:"latest_revision_number"`
	CreatedBy            *Member   `json:"created_by"`
	CreatedByAgent       *Agent    `json:"created_by_agent"`
	UpdatedBy            *Member   `json:"updated_by"`
	UpdatedByAgent       *Agent    `json:"updated_by_agent"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// revisionJSON is one Revision of an Issue document.
type revisionJSON struct {
	ID             uint64    `json:"id"`
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	ChangeSummary  *string   `json:"change_summary"`
	CreatedBy      *Member   `json:"created_by"`
	CreatedByAgent *Agent    `json:"created_by_agent"`
	CreatedAt      time.Time `json:"created_at"`
}

// memberMap names the Members among ids (0 left out) in one go.
func (c *Controller) memberMap(ctx context.Context, ids []uint64) (map[uint64]Member, error) {
	var want []uint64
	for _, id := range ids {
		if id != 0 {
			want = append(want, id)
		}
	}
	out := map[uint64]Member{}
	if len(want) == 0 {
		return out, nil
	}
	ms, err := c.members(ctx, want)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		out[m.ID] = m
	}
	return out, nil
}

func memberOf(ms map[uint64]Member, id uint64) *Member {
	if m, ok := ms[id]; ok {
		return &m
	}
	return nil
}

func (c *Controller) documentsJSON(ctx contractshttp.Context, ds []domain.IssueDocument) ([]documentJSON, error) {
	var actors []domain.Actor
	for _, d := range ds {
		actors = append(actors, d.CreatedBy, d.UpdatedBy)
	}
	ns, err := c.actorNames(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := make([]documentJSON, len(ds))
	for n, d := range ds {
		out[n] = documentJSON{
			ID: d.ID, Key: d.Key, Title: d.Title, Body: d.Body, Format: "markdown",
			LatestRevisionID: d.LatestRevisionID, LatestRevisionNumber: d.Latest,
			CreatedBy: ns.member(d.CreatedBy), CreatedByAgent: ns.agent(d.CreatedBy),
			UpdatedBy: ns.member(d.UpdatedBy), UpdatedByAgent: ns.agent(d.UpdatedBy),
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
		}
	}
	return out, nil
}

func (c *Controller) oneDocument(ctx contractshttp.Context, status int, d domain.IssueDocument, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.documentsJSON(ctx, []domain.IssueDocument{d})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"document": out[0]})
}

// ListDocuments answers the Issue's documents by key.
func (c *Controller) ListDocuments(ctx contractshttp.Context) contractshttp.Response {
	ds, err := c.service.Documents(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.documentsJSON(ctx, ds)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"documents": out})
}

func (c *Controller) ShowDocument(ctx contractshttp.Context) contractshttp.Response {
	d, err := c.service.Document(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), ctx.Request().Route("key"), c.visible(ctx))
	return c.oneDocument(ctx, contractshttp.StatusOK, d, err)
}

type documentRequest struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	ChangeSummary  string `json:"change_summary"`
	BaseRevisionID uint64 `json:"base_revision_id"`
}

// SaveDocument creates the document under {key} (201) or saves a new
// Revision of it on top of base_revision_id (200).
func (c *Controller) SaveDocument(ctx contractshttp.Context) contractshttp.Response {
	var req documentRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	in := app.DocumentInput{Title: req.Title, Body: req.Body, ChangeSummary: req.ChangeSummary, BaseRevisionID: req.BaseRevisionID}
	d, created, err := c.service.SaveDocument(ctx.Context(), c.guild(ctx), c.actor(ctx), ctx.Request().Route("id"), ctx.Request().Route("key"), in, c.visible(ctx))
	status := contractshttp.StatusOK
	if created {
		status = contractshttp.StatusCreated
	}
	return c.oneDocument(ctx, status, d, err)
}

// DeleteDocument deletes the document and every Revision of it.
func (c *Controller) DeleteDocument(ctx contractshttp.Context) contractshttp.Response {
	if err := c.service.DeleteDocument(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), ctx.Request().Route("key"), c.visible(ctx)); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

// ListRevisions answers the document's Revisions, newest first.
func (c *Controller) ListRevisions(ctx contractshttp.Context) contractshttp.Response {
	rs, err := c.service.Revisions(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), ctx.Request().Route("key"), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	authors := make([]domain.Actor, len(rs))
	for n, r := range rs {
		authors[n] = r.Author
	}
	ns, err := c.actorNames(ctx, authors)
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]revisionJSON, len(rs))
	for n, r := range rs {
		out[n] = revisionJSON{ID: r.ID, Number: r.Number, Title: r.Title, Body: r.Body, CreatedBy: ns.member(r.Author), CreatedByAgent: ns.agent(r.Author), CreatedAt: r.CreatedAt}
		if r.Summary != "" {
			out[n].ChangeSummary = &r.Summary
		}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"revisions": out})
}

// RestoreRevision saves the {revision} as the document's newest Revision.
func (c *Controller) RestoreRevision(ctx contractshttp.Context) contractshttp.Response {
	id, err := strconv.ParseUint(ctx.Request().Route("revision"), 10, 64)
	if err != nil || id == 0 {
		return notFound(ctx)
	}
	d, err := c.service.RestoreRevision(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), ctx.Request().Route("key"), id, c.visible(ctx))
	return c.oneDocument(ctx, contractshttp.StatusOK, d, err)
}
