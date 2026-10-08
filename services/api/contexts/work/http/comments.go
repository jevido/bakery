package http

import (
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// commentJSON is a Comment in an Issue's thread, by a Member (Author) or an
// Agent (AuthorAgent). A deleted one has an empty body; Edited is an edit
// after it was written.
type commentJSON struct {
	ID          uint64    `json:"id"`
	Body        string    `json:"body"`
	Deleted     bool      `json:"deleted"`
	Author      *Member   `json:"author"`
	AuthorAgent *Agent    `json:"author_agent"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Edited      bool      `json:"edited"`
}

// commentsJSON shows Comments with their authors' names, asked for in one
// go.
func (c *Controller) commentsJSON(ctx contractshttp.Context, cs []domain.Comment) ([]commentJSON, error) {
	authors := make([]domain.Actor, len(cs))
	for n, cm := range cs {
		authors[n] = cm.Author
	}
	names, err := c.actorNames(ctx, authors)
	if err != nil {
		return nil, err
	}
	out := make([]commentJSON, len(cs))
	for n, cm := range cs {
		out[n] = commentJSON{
			ID: cm.ID, Body: cm.Body, Deleted: cm.Deleted(), CreatedAt: cm.CreatedAt, UpdatedAt: cm.UpdatedAt,
			Edited: !cm.Deleted() && cm.UpdatedAt.After(cm.CreatedAt),
			Author: names.member(cm.Author), AuthorAgent: names.agent(cm.Author),
		}
	}
	return out, nil
}

type commentRequest struct {
	Body string `json:"body"`
}

// commentID reads the {comment} route parameter; anything but an id is
// no Comment.
func commentID(ctx contractshttp.Context) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Request().Route("comment"), 10, 64)
	return id, err == nil && id != 0
}

func (c *Controller) oneComment(ctx contractshttp.Context, status int, cm domain.Comment, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.commentsJSON(ctx, []domain.Comment{cm})
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Json(status, contractshttp.Json{"comment": out[0]})
}

// ListComments answers the Issue's thread, oldest first, deleted Comments
// in their place.
func (c *Controller) ListComments(ctx contractshttp.Context) contractshttp.Response {
	cs, err := c.service.Comments(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.commentsJSON(ctx, cs)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"comments": out})
}

func (c *Controller) WriteComment(ctx contractshttp.Context) contractshttp.Response {
	var req commentRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	cm, err := c.service.WriteComment(ctx.Context(), c.guild(ctx), c.actor(ctx), ctx.Request().Route("id"), req.Body, c.visible(ctx))
	return c.oneComment(ctx, contractshttp.StatusCreated, cm, err)
}

// EditComment replaces the body of one's own Comment.
func (c *Controller) EditComment(ctx contractshttp.Context) contractshttp.Response {
	id, ok := commentID(ctx)
	if !ok {
		return notFound(ctx)
	}
	var req commentRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	cm, err := c.service.EditComment(ctx.Context(), c.guild(ctx), c.actor(ctx), ctx.Request().Route("id"), id, req.Body, c.visible(ctx))
	return c.oneComment(ctx, contractshttp.StatusOK, cm, err)
}

// DeleteComment deletes one's own Comment; it stays in the thread as
// deleted.
func (c *Controller) DeleteComment(ctx contractshttp.Context) contractshttp.Response {
	id, ok := commentID(ctx)
	if !ok {
		return notFound(ctx)
	}
	if err := c.service.DeleteComment(ctx.Context(), c.guild(ctx), c.actor(ctx), ctx.Request().Route("id"), id, c.visible(ctx)); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
