// Package agents is what the router may use from the agents context: its
// routes (the Current guild's Agents, hiring and managing one, its Org
// chart, their Runs), the Desktop's routes for the Runs it runs, and the
// sweep of lost Runs (Start). It hears work's Decisions on hire_agent Approvals, Issues
// assigned to and commented on for its Agents, and Members leaving a Guild, names Agents as Issue Assignees for work, and keeps a Guild with Agents from being deleted. Nothing else in contexts/agents is for outside use.
package agents

import (
	"context"
	"errors"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/agents/app"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
	agentshttp "github.com/jevido/bakery/services/api/contexts/agents/http"
	"github.com/jevido/bakery/services/api/contexts/agents/infra"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/work"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		service = app.NewService(infra.Agents{}, infra.Runs{}, guildsOfAgents{}, workOfAgents{}, repositoriesOfAgents{}, infra.Budgets{})
		service.Logf = facades.Log().Errorf
		work.OnApprovalDecided("hire_agent", func(ctx context.Context, d work.ApprovalDecided) error {
			return service.Decided(ctx, app.Decision{GuildID: d.GuildID, AgentID: d.AgentID, DeciderID: d.DeciderID, Approved: d.Approved})
		})
		work.OnAgentNames(service.Names)
		work.OnIssueAssigned(func(ctx context.Context, e work.IssueAssigned) error {
			return service.IssueAssigned(ctx, e.GuildID, e.IssueID, e.AgentID, e.ActorID)
		})
		work.OnIssueCommented(func(ctx context.Context, e work.IssueCommented) error {
			return service.IssueCommented(ctx, e.GuildID, e.IssueID, e.AgentID, e.CommentID, e.ActorID)
		})
		work.OnAgentAssignees(func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]work.AssigneeAgent, error) {
			as, err := service.Assignees(ctx, guildID, ids)
			if err != nil {
				return nil, err
			}
			out := make(map[uint64]work.AssigneeAgent, len(as))
			for id, a := range as {
				out[id] = work.AssigneeAgent{Name: a.Name, Icon: string(a.Icon), Terminated: a.Status == domain.Terminated}
			}
			return out, nil
		})
		work.OnRunLive(service.LiveRuns)
		identity.OnRunKey(func(ctx context.Context, keyHash string) (identity.RunKeyHolder, bool, error) {
			h, ok, err := service.RunKeyHolder(ctx, keyHash)
			return identity.RunKeyHolder(h), ok, err
		})
		guilds.OnGuildDeleting("agents", service.HasAgents)
		guilds.OnMemberLeaving(service.HirerLeft)
		projects.OnProjectDeleted(service.ForgetProject)
	})
	return service
}

// repositoriesOfAgents is projects' answer about an Issue's Application and
// the names of a Guild's Projects, in agents' terms.
type repositoriesOfAgents struct{}

func (repositoriesOfAgents) ApplicationRepository(ctx context.Context, guildID, applicationID uint64) (app.GitRepository, bool, error) {
	r, ok, err := projects.ApplicationRepository(ctx, guildID, applicationID)
	return app.GitRepository(r), ok, err
}

func (repositoriesOfAgents) ProjectNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	return projects.ProjectNames(ctx, guildID, ids)
}

// guildsOfAgents is guilds' answer about Agent memberships, in agents'
// terms.
type guildsOfAgents struct{}

func (guildsOfAgents) JoinAgent(ctx context.Context, guildID, hirerID, agentID uint64, hirerPerms []string, roleIDs []uint64) error {
	err := guilds.JoinAgent(ctx, guildID, hirerID, agentID, hirerPerms, roleIDs)
	if guilds.AgentRoleRefused(err) {
		return &domain.FieldError{Field: "role_ids", Message: err.Error()}
	}
	if errors.Is(err, guilds.ErrNotAMember) {
		return app.ErrHirerNotMember
	}
	return err
}

// refused turns guilds refusing a Role into a 422 on role_id.
func refused(err error) error {
	if guilds.AgentRoleRefused(err) {
		return &domain.FieldError{Field: "role_id", Message: err.Error()}
	}
	return err
}

func (guildsOfAgents) AssignAgentRole(ctx context.Context, guildID uint64, actor app.Actor, agentID, roleID uint64) error {
	return refused(guilds.AssignAgentRole(ctx, guildID, actor.ID, actor.Permissions, agentID, roleID))
}

func (guildsOfAgents) RemoveAgentRole(ctx context.Context, guildID uint64, actor app.Actor, agentID, roleID uint64) error {
	return refused(guilds.RemoveAgentRole(ctx, guildID, actor.ID, actor.Permissions, agentID, roleID))
}

func (guildsOfAgents) RankAbove(ctx context.Context, guildID, actorID, memberID uint64) (bool, error) {
	return guilds.RankAbove(ctx, guildID, actorID, memberID)
}

func (guildsOfAgents) LeaveAgent(ctx context.Context, guildID, agentID uint64) error {
	return guilds.LeaveAgent(ctx, guildID, agentID)
}

func (guildsOfAgents) AgentRoles(ctx context.Context, guildID, agentID uint64) ([]app.Role, error) {
	rs, err := guilds.AgentRoles(ctx, guildID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]app.Role, len(rs))
	for i, r := range rs {
		out[i] = app.Role{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position}
	}
	return out, nil
}

func (guildsOfAgents) GuildName(ctx context.Context, guildID uint64) (string, error) {
	return guilds.GuildName(ctx, guildID)
}

func (guildsOfAgents) IssuePrefix(ctx context.Context, guildID uint64) (string, error) {
	return guilds.IssuePrefix(ctx, guildID)
}

func (guildsOfAgents) RoleNames(ctx context.Context, guildID uint64, ids []uint64) ([]string, error) {
	return guilds.RoleNames(ctx, guildID, ids)
}

// workOfAgents is work's hire_agent Approvals and Activity, in agents'
// terms.
type workOfAgents struct{}

func (workOfAgents) RequestHireApproval(ctx context.Context, guildID, hirerID uint64, r app.HireRequest) (uint64, error) {
	a := r.Agent
	return work.RequestApproval(ctx, guildID, hirerID, work.HireAgentRequest{
		AgentID: a.ID, Name: a.Name, Job: string(a.Job), Title: a.Title, Icon: string(a.Icon), Capabilities: a.Capabilities,
		ManagerID: a.ManagerID, ManagerName: r.ManagerName, Roles: r.Roles,
	})
}

func (workOfAgents) RecordActivity(ctx context.Context, e app.Activity) error {
	return work.RecordActivity(ctx, work.AgentActivity{
		GuildID: e.GuildID, ActorID: e.ActorID, AgentID: e.AgentID, Entity: e.Entity, EntityID: e.EntityID,
		Action: e.Action, AgentName: e.AgentName, Details: e.Details,
	})
}

func (workOfAgents) RequestBudgetOverride(ctx context.Context, b app.BudgetSummary, i domain.BudgetIncident) (uint64, error) {
	r := work.BudgetOverrideRequest{
		BudgetID: b.ID, ScopeType: string(b.Scope), ScopeID: b.ScopeID, ScopeName: b.ScopeName, Metric: string(b.Metric),
		Window: string(b.Window), Threshold: string(i.Threshold), Amount: i.Amount, Observed: i.Observed, WarnPercent: b.WarnPercent,
	}
	if !i.WindowStart.IsZero() {
		r.WindowStart, r.WindowEnd = &i.WindowStart, &i.WindowEnd
	}
	return work.RequestBudgetOverride(ctx, b.GuildID, r)
}

func (workOfAgents) DecideBudgetOverride(ctx context.Context, guildID, memberID, approvalID uint64, approved bool, note string) error {
	return work.DecideBudgetOverride(ctx, guildID, memberID, approvalID, approved, note)
}

func (workOfAgents) CancelApproval(ctx context.Context, guildID, actorID, approvalID uint64) error {
	return work.CancelApproval(ctx, guildID, actorID, approvalID)
}

func (workOfAgents) UnassignAgent(ctx context.Context, guildID, agentID, actorID uint64) error {
	return work.UnassignAgent(ctx, guildID, agentID, actorID)
}

func (workOfAgents) IssueForRun(ctx context.Context, guildID, issueID uint64) (app.IssueBrief, bool, error) {
	i, ok, err := work.IssueForRun(ctx, guildID, issueID)
	return app.IssueBrief(i), ok, err
}

func (workOfAgents) OpenIssuesOfAgent(ctx context.Context, guildID, agentID uint64) ([]app.IssueBrief, error) {
	is, err := work.OpenIssuesOfAgent(ctx, guildID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]app.IssueBrief, len(is))
	for n, i := range is {
		out[n] = app.IssueBrief(i)
	}
	return out, nil
}

func (workOfAgents) InboxOfAgent(ctx context.Context, guildID, agentID uint64) ([]app.InboxIssue, error) {
	is, err := work.InboxOfAgent(ctx, guildID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]app.InboxIssue, len(is))
	for n, i := range is {
		out[n] = app.InboxIssue(i)
	}
	return out, nil
}

func (workOfAgents) CommentsForRun(ctx context.Context, guildID uint64, ids []uint64) ([]app.RunComment, error) {
	cs, err := work.CommentsForRun(ctx, guildID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]app.RunComment, len(cs))
	for n, c := range cs {
		out[n] = app.RunComment(c)
	}
	return out, nil
}

func memberNames(ctx context.Context, ids []uint64) ([]agentshttp.Named, error) {
	ms, err := identity.Members(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]agentshttp.Named, len(ms))
	for i, m := range ms {
		out[i] = agentshttp.Named{ID: m.ID, Name: m.Name}
	}
	return out, nil
}

// runInGuild answers 404 for a route whose {id} Run is another Guild's.
var runInGuild = guilds.Owns("run", func(ctx context.Context, id, guildID uint64) (bool, error) {
	return svc().RunInGuild(ctx, id, guildID)
})

// agentInGuild answers 404 for a route whose {id} Agent is another
// Guild's.
var agentInGuild = guilds.Owns("agent", func(ctx context.Context, id, guildID uint64) (bool, error) {
	return svc().AgentInGuild(ctx, id, guildID)
})

// controller builds the Agents API controller, wired to guilds and
// identity; Routes and StreamRoutes each need one.
func controller() *agentshttp.Controller {
	c := agentshttp.NewController(svc())
	c.Guild, c.Member, c.Permissions, c.Members = guilds.Current, guilds.MemberID, guilds.Permissions, memberNames
	c.InstanceAdmin, c.Visible, c.Desktops = guilds.InstanceAdmin, guilds.VisibleProjects, identity.DesktopNames
	c.Agent, c.Run = guilds.AgentID, guilds.RunID
	c.Shutdown = shutdown
	return c
}

// Routes registers the Current guild's Agents API: reading needs
// view_resources, hiring hire_agents, managing an Agent (and starting or
// cancelling its Runs) hire_agents and being its Hirer or ranking above
// them. An Agent's Run key may read the Agents, the Org chart and the
// Runs, its own only (guilds.AuthAgents); every change stays a
// Member's. Registering them also subscribes agents to work's hire_agent
// Decisions, Members leaving and Guild deletions.
func Routes(r route.Router) {
	c := controller()
	view := guilds.Can("view_resources")
	// Who the Agent is and what is assigned to it are its own to know,
	// whatever its Roles, as Paperclip's /agents/me; a person gets 403.
	r.Middleware(guilds.AuthAgents).Group(func(r route.Router) {
		r.Get("/api/agents/me", c.ShowMe)
		r.Get("/api/agents/me/inbox", c.ShowMyInbox)
	})
	r.Middleware(guilds.AuthAgents, view).Group(func(r route.Router) {
		r.Get("/api/agents", c.ListAgents)
		r.Get("/api/org", c.ShowOrg)
		r.Get("/api/runs", c.ListRuns)
	})
	// Costs and Budgets are the Board's to read, as Paperclip's: not open
	// to Agents.
	r.Middleware(guilds.Auth, view).Group(func(r route.Router) {
		r.Get("/api/costs/summary", c.CostSummary)
		r.Get("/api/costs/by-agent", c.CostsByAgent)
		r.Get("/api/costs/by-project", c.CostsByProject)
		r.Get("/api/budgets/overview", c.BudgetOverview)
	})
	r.Middleware(guilds.AuthAgents, runInGuild, view).Group(func(r route.Router) {
		r.Get("/api/runs/{id}", c.ShowRun)
		r.Get("/api/runs/{id}/events", c.ListRunEvents)
	})
	r.Middleware(guilds.Auth, guilds.Can("manage_budgets")).Put("/api/budgets", c.SetBudget)
	r.Middleware(guilds.Auth, runInGuild, guilds.Can("hire_agents")).Post("/api/runs/{id}/cancel", c.CancelRun)
	r.Middleware(guilds.Auth, guilds.Can("hire_agents")).Post("/api/agents", c.HireAgent)
	r.Middleware(guilds.AuthAgents, agentInGuild, view).Get("/api/agents/{id}", c.ShowAgent)
	r.Middleware(guilds.Auth, agentInGuild, guilds.Can("hire_agents")).Group(func(r route.Router) {
		r.Patch("/api/agents/{id}", c.EditAgent)
		r.Post("/api/agents/{id}/pause", c.PauseAgent)
		r.Post("/api/agents/{id}/resume", c.ResumeAgent)
		r.Post("/api/agents/{id}/terminate", c.TerminateAgent)
		r.Put("/api/agents/{id}/roles/{role_id}", c.AddAgentRole)
		r.Delete("/api/agents/{id}/roles/{role_id}", c.RemoveAgentRole)
		r.Post("/api/agents/{id}/runs", c.StartRun)
		r.Post("/api/agents/{id}/heartbeat", c.RunHeartbeat)
	})
	d := desktopController()
	r.Middleware(identity.DesktopOnly).Group(func(r route.Router) {
		r.Get("/api/desktop/runs", d.ListRuns)
		r.Post("/api/runs/{id}/claim", d.ClaimRun)
		r.Post("/api/runs/{id}/events", d.AppendRunEvents)
		r.Post("/api/runs/{id}/lease", d.KeepRunLease)
		r.Post("/api/runs/{id}/finish", d.FinishRun)
	})
}

// shutdown closes when the context Start was given ends, so open Desktop
// streams let the API stop.
var shutdown = make(chan struct{})

func desktopController() *agentshttp.DesktopController {
	d := agentshttp.NewDesktopController(svc())
	d.Desktop, d.Shutdown = identity.DesktopOf, shutdown
	d.SeeDesktop = func(ctx contractshttp.Context, id uint64) error { return identity.SeeDesktop(ctx.Context(), id) }
	return d
}

// StreamRoutes registers the Desktop's stream of its Member's Runs, with
// a Desktop key, and a Run's live Transcript for the dashboard, both
// outside the request timeout.
func StreamRoutes(r route.Router) {
	r.Middleware(identity.DesktopOnly).Get("/api/desktop/runs/stream", desktopController().StreamRuns)
	r.Middleware(guilds.Auth, runInGuild, guilds.Can("view_resources")).Get("/api/runs/{id}/stream", controller().RunStream)
}

// sweepEvery is how often Runs whose Lease ran out and due Heartbeats
// are looked for.
const sweepEvery = 30 * time.Second

// Start ends, every 30 seconds until ctx ends, the running Runs whose
// Desktop stopped reporting as lost, queueing each again, and then starts
// the Heartbeats that are due.
func Start(ctx context.Context) {
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()
	go func() {
		t := time.NewTicker(sweepEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := svc().SweepLostRuns(ctx); err != nil {
					facades.Log().Errorf("agents: sweeping lost runs: %v", err)
				}
				if _, err := svc().TickHeartbeats(ctx); err != nil {
					facades.Log().Errorf("agents: ticking heartbeats: %v", err)
				}
			}
		}
	}()
}
