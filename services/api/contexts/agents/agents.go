// Package agents is what the router may use from the agents context: its
// routes (the Current guild's Agents, hiring one, its Org chart). It hears
// work's Decisions on hire_agent Approvals and keeps a Guild with Agents
// from being deleted. Nothing else in contexts/agents is for outside use.
package agents

import (
	"context"
	"sync"

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
		service = app.NewService(infra.Agents{}, guildsOfAgents{}, workOfAgents{})
		service.Logf = facades.Log().Errorf
		work.OnApprovalDecided("hire_agent", func(ctx context.Context, d work.ApprovalDecided) error {
			return service.Decided(ctx, app.Decision{GuildID: d.GuildID, AgentID: d.AgentID, DeciderID: d.DeciderID, Approved: d.Approved})
		})
		work.OnAgentNames(service.Names)
		guilds.OnGuildDeleting("agents", service.HasAgents)
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
	return err
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

// agentInGuild answers 404 for a route whose {id} Agent is another
// Guild's.
var agentInGuild = guilds.Owns("agent", func(ctx context.Context, id, guildID uint64) (bool, error) {
	return svc().AgentInGuild(ctx, id, guildID)
})

// Routes registers the Current guild's Agents API: reading needs
// view_resources, hiring hire_agents. Registering them also subscribes
// agents to work's hire_agent Decisions and to Guild deletions.
func Routes(r route.Router) {
	c := agentshttp.NewController(svc())
	c.Guild, c.Member, c.Permissions, c.Members = guilds.Current, guilds.MemberID, guilds.Permissions, memberNames
	view := guilds.Can("view_resources")
	r.Middleware(guilds.Auth, view).Group(func(r route.Router) {
		r.Get("/api/agents", c.ListAgents)
		r.Get("/api/org", c.ShowOrg)
	})
	r.Middleware(guilds.Auth, guilds.Can("hire_agents")).Post("/api/agents", c.HireAgent)
	r.Middleware(guilds.Auth, agentInGuild, view).Get("/api/agents/{id}", c.ShowAgent)
}
