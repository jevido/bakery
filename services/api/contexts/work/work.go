// Package work is what the router may use from the work context: its
// routes. Nothing else in contexts/work is for outside use.
package work

import (
	"context"
	"sync"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/identity"
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
		service = app.NewService(infra.Goals{}, members{})
		guilds.OnGuildDeleting("goals", func(ctx context.Context, guildID uint64) (bool, error) {
			gs, err := service.Goals(ctx, guildID)
			return len(gs) > 0, err
		})
	})
	return service
}

// members is guilds' and identity's answer about Members, in work's terms.
type members struct{}

func (members) IsMember(ctx context.Context, guildID, memberID uint64) (bool, error) {
	return guilds.IsMember(ctx, guildID, memberID)
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

// Routes registers the Current guild's Goals API: reading needs
// view_resources, changing manage_work.
func Routes(r route.Router) {
	c := workhttp.NewController(svc(), guilds.Current, memberNames)
	view, manage := guilds.Can("view_resources"), guilds.Can("manage_work")
	r.Middleware(guilds.Auth, view).Get("/api/goals", c.ListGoals)
	r.Middleware(guilds.Auth, manage).Post("/api/goals", c.CreateGoal)
	r.Middleware(guilds.Auth, goalInGuild, view).Get("/api/goals/{id}", c.ShowGoal)
	r.Middleware(guilds.Auth, goalInGuild, manage).Group(func(r route.Router) {
		r.Patch("/api/goals/{id}", c.UpdateGoal)
		r.Delete("/api/goals/{id}", c.DeleteGoal)
	})
}
