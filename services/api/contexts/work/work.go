// Package work is what the router may use from the work context: its
// routes (Goals, Issues and Comments). Nothing else in contexts/work is for outside use.
package work

import (
	"context"
	"sync"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	workhttp "github.com/jevido/bakery/services/api/contexts/work/http"
	"github.com/jevido/bakery/services/api/contexts/work/infra"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		service = app.NewService(infra.Goals{}, infra.Issues{}, infra.Comments{}, guildsOfWork{}, projectsOfWork{})
		guilds.OnGuildDeleting("goals", func(ctx context.Context, guildID uint64) (bool, error) {
			gs, err := service.Goals(ctx, guildID)
			return len(gs) > 0, err
		})
		guilds.OnGuildDeleting("issues", service.HasIssues)
		projects.OnProjectDeleted(service.ForgetProject)
	})
	return service
}

// guildsOfWork is guilds' answer about Members and Issue prefixes, in
// work's terms.
type guildsOfWork struct{}

func (guildsOfWork) IsMember(ctx context.Context, guildID, memberID uint64) (bool, error) {
	return guilds.IsMember(ctx, guildID, memberID)
}

func (guildsOfWork) IssuePrefix(ctx context.Context, guildID uint64) (string, error) {
	return guilds.IssuePrefix(ctx, guildID)
}

// projectsOfWork is projects' answer about Project names.
type projectsOfWork struct{}

func (projectsOfWork) ProjectNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	return projects.ProjectNames(ctx, guildID, ids)
}

func memberNames(ctx context.Context, ids []uint64) ([]workhttp.Member, error) {
	ms, err := identity.Members(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]workhttp.Member, len(ms))
	for i, m := range ms {
		out[i] = workhttp.Member{ID: m.ID, Name: m.Name}
	}
	return out, nil
}

// goalInGuild answers 404 for a route whose {id} Goal is another Guild's.
var goalInGuild = guilds.Owns("goal", func(ctx context.Context, id, guildID uint64) (bool, error) {
	return svc().GoalInGuild(ctx, id, guildID)
})

// Routes registers the Current guild's Goals, Issues and Comments API:
// reading needs view_resources, changing manage_work, and changing a
// Comment also being its author. An Issue's {id} is its id or its
// Issue identifier, so the service, not guilds.Owns, answers 404 for one
// outside the Current guild or in a Project the request may not view.
func Routes(r route.Router) {
	c := workhttp.NewController(svc(), guilds.Current, memberNames)
	c.Visible, c.Member = guilds.VisibleProjects, guilds.MemberID
	view, manage := guilds.Can("view_resources"), guilds.Can("manage_work")
	r.Middleware(guilds.Auth, view).Get("/api/goals", c.ListGoals)
	r.Middleware(guilds.Auth, manage).Post("/api/goals", c.CreateGoal)
	r.Middleware(guilds.Auth, goalInGuild, view).Get("/api/goals/{id}", c.ShowGoal)
	r.Middleware(guilds.Auth, goalInGuild, manage).Group(func(r route.Router) {
		r.Patch("/api/goals/{id}", c.UpdateGoal)
		r.Delete("/api/goals/{id}", c.DeleteGoal)
	})
	r.Middleware(guilds.Auth, view).Get("/api/issues", c.ListIssues)
	r.Middleware(guilds.Auth, manage).Post("/api/issues", c.CreateIssue)
	r.Middleware(guilds.Auth, view).Get("/api/issues/{id}", c.ShowIssue)
	r.Middleware(guilds.Auth, manage).Group(func(r route.Router) {
		r.Patch("/api/issues/{id}", c.UpdateIssue)
		r.Delete("/api/issues/{id}", c.DeleteIssue)
	})
	r.Middleware(guilds.Auth, view).Get("/api/issues/{id}/comments", c.ListComments)
	r.Middleware(guilds.Auth, manage).Group(func(r route.Router) {
		r.Post("/api/issues/{id}/comments", c.WriteComment)
		r.Patch("/api/issues/{id}/comments/{comment}", c.EditComment)
		r.Delete("/api/issues/{id}/comments/{comment}", c.DeleteComment)
	})
}
