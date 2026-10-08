package http

import (
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
)

type meGuildJSON struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	IssuePrefix string `json:"issue_prefix"`
}

type meRunJSON struct {
	ID               uint64        `json:"id"`
	Issue            *runIssueJSON `json:"issue"`
	InvocationSource string        `json:"invocation_source"`
	WakeReason       string        `json:"wake_reason"`
}

type chainJSON struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Job   string `json:"job"`
	Title string `json:"title"`
}

type inboxProjectJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type inboxIssueJSON struct {
	ID            uint64            `json:"id"`
	Identifier    string            `json:"identifier"`
	Title         string            `json:"title"`
	Status        string            `json:"status"`
	Priority      string            `json:"priority"`
	Project       *inboxProjectJSON `json:"project"`
	GoalID        *uint64           `json:"goal_id"`
	ParentID      *uint64           `json:"parent_id"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CheckoutRunID *uint64           `json:"checkout_run_id"`
}

func idOrNil(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

// notAnAgent answers 403 to a person on a route only an Agent's Run key
// may use.
func notAnAgent(ctx contractshttp.Context) contractshttp.Response {
	return respond.Error(ctx, contractshttp.StatusForbidden, "only an agent's run key may use this")
}

// ShowMe answers the Agent asking through its Run key: itself, its Guild,
// its Run, its Chain of command and its Permissions in the Guild.
func (c *Controller) ShowMe(ctx contractshttp.Context) contractshttp.Response {
	agentID := c.agent(ctx)
	if agentID == 0 {
		return notAnAgent(ctx)
	}
	guild := c.Guild(ctx)
	me, err := c.service.Me(ctx.Context(), guild, agentID, c.Run(ctx), c.visible(ctx))
	if err != nil {
		return runFail(ctx, err)
	}
	a, err := c.oneAgent(ctx, me.Agent)
	if err != nil {
		return fail(ctx, err)
	}
	run := meRunJSON{ID: me.Run.ID, InvocationSource: string(me.Run.InvocationSource), WakeReason: string(me.Run.WakeReason)}
	if me.Issue != nil {
		run.Issue = &runIssueJSON{ID: me.Issue.ID, Identifier: me.Issue.Identifier, Title: me.Issue.Title}
	}
	chain := make([]chainJSON, len(me.Chain))
	for i, m := range me.Chain {
		chain[i] = chainJSON{ID: m.ID, Name: m.Name, Job: string(m.Job), Title: m.Title}
	}
	perms := c.Permissions(ctx)
	if perms == nil {
		perms = []string{}
	}
	return ctx.Response().Success().Json(contractshttp.Json{
		"agent": a, "guild": meGuildJSON{ID: guild, Name: me.GuildName, IssuePrefix: me.IssuePrefix},
		"run": run, "chain_of_command": chain, "permissions": perms,
	})
}

// ShowMyInbox answers the Issues assigned to the Agent asking through its
// Run key that it can work on, in progress first, as Paperclip's
// inbox-lite.
func (c *Controller) ShowMyInbox(ctx contractshttp.Context) contractshttp.Response {
	agentID := c.agent(ctx)
	if agentID == 0 {
		return notAnAgent(ctx)
	}
	is, err := c.service.Inbox(ctx.Context(), c.Guild(ctx), agentID, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out := make([]inboxIssueJSON, len(is))
	for n, i := range is {
		j := inboxIssueJSON{
			ID: i.ID, Identifier: i.Identifier, Title: i.Title, Status: i.Status, Priority: i.Priority,
			GoalID: idOrNil(i.GoalID), ParentID: idOrNil(i.ParentID), UpdatedAt: i.UpdatedAt.UTC(),
			CheckoutRunID: idOrNil(i.CheckoutRunID),
		}
		if i.ProjectID != 0 {
			j.Project = &inboxProjectJSON{ID: i.ProjectID, Name: i.ProjectName}
		}
		out[n] = j
	}
	return ctx.Response().Success().Json(contractshttp.Json{"issues": out})
}
