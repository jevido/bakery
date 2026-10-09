package http

import (
	contractshttp "github.com/goravel/framework/contracts/http"
)

// ShowBoardChat answers the Guild's Board chat, or null before anyone
// opened it.
func (c *Controller) ShowBoardChat(ctx contractshttp.Context) contractshttp.Response {
	i, found, err := c.service.BoardChat(ctx.Context(), c.guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	if !found {
		return ctx.Response().Success().Json(contractshttp.Json{"issue": nil})
	}
	return c.oneIssue(ctx, contractshttp.StatusOK, i, nil)
}

// OpenBoardChat answers the Guild's Board chat, opening it the first
// time.
func (c *Controller) OpenBoardChat(ctx contractshttp.Context) contractshttp.Response {
	i, err := c.service.OpenBoardChat(ctx.Context(), c.guild(ctx), c.actor(ctx))
	return c.oneIssue(ctx, contractshttp.StatusOK, i, err)
}
