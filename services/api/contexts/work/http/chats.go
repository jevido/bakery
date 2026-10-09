package http

import (
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"
)

// conversationJSON is what makes an Issue a Conversation, on the wire.
// The Board chat has no member and its agent is the Guild's CEO, null
// when it has none.
type conversationJSON struct {
	Board             bool    `json:"board"`
	Agent             *Agent  `json:"agent"`
	MemberID          *uint64 `json:"member_id"`
	State             string  `json:"state"`
	BoundaryCommentID *uint64 `json:"boundary_comment_id"`
}

// chatAgent reads the {agent_id} route parameter; anything but an id is
// no Agent.
func chatAgent(ctx contractshttp.Context) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Request().Route("agent_id"), 10, 64)
	return id, err == nil && id != 0
}

// ListChats answers the asking Member's Conversations, most recently
// updated first.
func (c *Controller) ListChats(ctx contractshttp.Context) contractshttp.Response {
	is, err := c.service.Conversations(ctx.Context(), c.guild(ctx), c.Member(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.issuesJSON(ctx, is, false)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"issues": out})
}

// ShowChat answers the asking Member's Conversation with the Agent, or
// null when they have none yet.
func (c *Controller) ShowChat(ctx contractshttp.Context) contractshttp.Response {
	agentID, ok := chatAgent(ctx)
	if !ok {
		return notFound(ctx)
	}
	i, found, err := c.service.ConversationWith(ctx.Context(), c.guild(ctx), c.Member(ctx), agentID)
	if err != nil {
		return fail(ctx, err)
	}
	if !found {
		return ctx.Response().Success().Json(contractshttp.Json{"issue": nil})
	}
	return c.oneIssue(ctx, contractshttp.StatusOK, i, nil)
}

// OpenChat answers the asking Member's Conversation with the Agent,
// opening it the first time.
func (c *Controller) OpenChat(ctx contractshttp.Context) contractshttp.Response {
	agentID, ok := chatAgent(ctx)
	if !ok {
		return notFound(ctx)
	}
	i, err := c.service.OpenConversation(ctx.Context(), c.guild(ctx), c.actor(ctx), agentID)
	return c.oneIssue(ctx, contractshttp.StatusOK, i, err)
}
