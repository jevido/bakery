package routes

import (
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	goravelgin "github.com/goravel/gin"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/databases"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/notifications"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/routing"
	"github.com/jevido/bakery/services/api/contexts/servers"
	"github.com/jevido/bakery/services/api/contexts/services"
	"github.com/jevido/bakery/services/api/contexts/work"
)

// requestTimeout bounds every route except the live log streams (see
// config/http.go for why the global timeout is off).
const requestTimeout = 15 * time.Second

func Api() {
	facades.Route().Middleware(goravelgin.Timeout(requestTimeout)).Group(func(r route.Router) {
		r.Get("/api/health", health)
		identity.Routes(r)
		guilds.Routes(r)
		projects.Routes(r)
		deployments.Routes(r)
		routing.Routes(r)
		databases.Routes(r)
		services.Routes(r)
		servers.Routes(r)
		notifications.Routes(r)
		work.Routes(r)
	})
	deployments.StreamRoutes(facades.Route())
	databases.StreamRoutes(facades.Route())
	services.StreamRoutes(facades.Route())
	servers.LongRoutes(facades.Route())
}

func health(ctx http.Context) http.Response {
	if err := facades.DB().Select(&[]struct{ One int }{}, "SELECT 1 AS one"); err != nil {
		return ctx.Response().Json(http.StatusServiceUnavailable, http.Json{"ok": false, "error": "database unreachable"})
	}
	return ctx.Response().Json(http.StatusOK, http.Json{"ok": true})
}
