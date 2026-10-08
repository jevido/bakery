// Package agents is what the router may use from the agents context: its
// routes (the Current guild's Agents, hiring and managing one, its Org
// chart, their Runs), the Desktop's routes for the Runs it runs, and the
// sweep of lost Runs (Start). It hears work's Decisions on hire_agent Approvals and Members
// leaving a Guild, names Agents as Issue Assignees for work, and keeps a Guild with Agents from being deleted. Nothing else in contexts/agents is for outside use.
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
	"github.com/jevido/bakery/services/api/contexts/work"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		service = app.NewService(infra.Agents{}, infra.Runs{}, guildsOfAgents{}, workOfAgents{})
		service.Logf = facades.Log().Errorf
		work.OnApprovalDecided("hire_agent", func(ctx context.Context, d work.ApprovalDecided) error {
			return service.Decided(ctx, app.Decision{GuildID: d.GuildID, AgentID: d.AgentID, DeciderID: d.DeciderID, Approved: d.Approved})
		})
		work.OnAgentNames(service.Names)
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
		guilds.OnGuildDeleting("agents", service.HasAgents)
		guilds.OnMemberLeaving(service.HirerLeft)
	})
	return service
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
		GuildID: e.GuildID, ActorID: e.ActorID, AgentID: e.AgentID, Action: e.Action, AgentName: e.AgentName, Details: e.Details,
	})
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

// Routes registers the Current guild's Agents API: reading needs
// view_resources, hiring hire_agents, managing an Agent (and starting or
// cancelling its Runs) hire_agents and being its Hirer or ranking above
// them. Registering them also subscribes
// agents to work's hire_agent Decisions, Members leaving and Guild
// deletions.
func Routes(r route.Router) {
	c := agentshttp.NewController(svc())
	c.Guild, c.Member, c.Permissions, c.Members = guilds.Current, guilds.MemberID, guilds.Permissions, memberNames
	c.InstanceAdmin, c.Visible, c.Desktops = guilds.InstanceAdmin, guilds.VisibleProjects, identity.DesktopNames
	view := guilds.Can("view_resources")
	r.Middleware(guilds.Auth, view).Group(func(r route.Router) {
		r.Get("/api/agents", c.ListAgents)
		r.Get("/api/org", c.ShowOrg)
		r.Get("/api/runs", c.ListRuns)
	})
	r.Middleware(guilds.Auth, runInGuild, view).Group(func(r route.Router) {
		r.Get("/api/runs/{id}", c.ShowRun)
		r.Get("/api/runs/{id}/events", c.ListRunEvents)
	})
	r.Middleware(guilds.Auth, runInGuild, guilds.Can("hire_agents")).Post("/api/runs/{id}/cancel", c.CancelRun)
	r.Middleware(guilds.Auth, guilds.Can("hire_agents")).Post("/api/agents", c.HireAgent)
	r.Middleware(guilds.Auth, agentInGuild, view).Get("/api/agents/{id}", c.ShowAgent)
	r.Middleware(guilds.Auth, agentInGuild, guilds.Can("hire_agents")).Group(func(r route.Router) {
		r.Patch("/api/agents/{id}", c.EditAgent)
		r.Post("/api/agents/{id}/pause", c.PauseAgent)
		r.Post("/api/agents/{id}/resume", c.ResumeAgent)
		r.Post("/api/agents/{id}/terminate", c.TerminateAgent)
		r.Put("/api/agents/{id}/roles/{role_id}", c.AddAgentRole)
		r.Delete("/api/agents/{id}/roles/{role_id}", c.RemoveAgentRole)
		r.Post("/api/agents/{id}/runs", c.StartRun)
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
// a Desktop key, outside the request timeout.
func StreamRoutes(r route.Router) {
	r.Middleware(identity.DesktopOnly).Get("/api/desktop/runs/stream", desktopController().StreamRuns)
}

// sweepEvery is how often Runs whose Lease ran out are looked for.
const sweepEvery = 30 * time.Second

// Start ends, every 30 seconds until ctx ends, the running Runs whose
// Desktop stopped reporting as lost, queueing each again.
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
			}
		}
	}()
}
