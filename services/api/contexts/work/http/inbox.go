package http

import (
	contractshttp "github.com/goravel/framework/contracts/http"
)

// MarkRead sets the asking Member's Read mark on the Issue.
func (c *Controller) MarkRead(ctx contractshttp.Context) contractshttp.Response {
	m, err := c.service.MarkRead(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"issue_id": m.IssueID, "last_read_at": m.At.UTC()})
}

// MarkUnread removes the asking Member's Read mark from the Issue.
func (c *Controller) MarkUnread(ctx contractshttp.Context) contractshttp.Response {
	if err := c.service.MarkUnread(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), c.visible(ctx)); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

// ArchiveFromInbox sets the asking Member's Inbox archive on the Issue.
func (c *Controller) ArchiveFromInbox(ctx contractshttp.Context) contractshttp.Response {
	a, err := c.service.ArchiveFromInbox(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"issue_id": a.IssueID, "archived_at": a.At.UTC()})
}

// UnarchiveFromInbox removes the asking Member's Inbox archive from the Issue.
func (c *Controller) UnarchiveFromInbox(ctx contractshttp.Context) contractshttp.Response {
	if err := c.service.UnarchiveFromInbox(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), c.visible(ctx)); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}
