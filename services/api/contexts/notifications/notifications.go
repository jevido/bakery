// Package notifications is what the router and the boot code may use from
// the notifications context: its routes, Start (the dispatcher and the
// events it subscribes to) and DashboardURL. Nothing else in
// contexts/notifications is for outside use.
package notifications

import (
	"context"
	"strings"
	"sync"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/notifications/app"
	notificationshttp "github.com/jevido/bakery/services/api/contexts/notifications/http"
	"github.com/jevido/bakery/services/api/contexts/notifications/infra"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		cfg := facades.Config()
		service = app.NewService(infra.Store{}, infra.Senders{TelegramAPI: cfg.GetString("bakery.notifications.telegram_api")})
		service.DashboardURL = DashboardURL()
		service.Log = facades.Log().Errorf
	})
	return service
}

// Routes registers the Notification channels API, admins only: channels
// hold credentials of other systems.
func Routes(r route.Router) {
	c := notificationshttp.NewController(svc())
	r.Middleware(identity.Auth, identity.Admin).Group(func(r route.Router) {
		r.Get("/api/notification-event-kinds", c.EventKinds)
		r.Get("/api/notification-channels", c.List)
		r.Post("/api/notification-channels", c.Create)
		r.Get("/api/notification-channels/{id}", c.Show)
		r.Patch("/api/notification-channels/{id}", c.Update)
		r.Delete("/api/notification-channels/{id}", c.Delete)
		r.Post("/api/notification-channels/{id}/test", c.Test)
		r.Get("/api/notification-channels/{id}/deliveries", c.Deliveries)
	})
}

// DashboardURL is where the dashboard is reached, without a trailing slash:
// bakery.dashboard.url, else https://<bakery.dashboard.domain>, else the
// Vite dev server.
func DashboardURL() string {
	cfg := facades.Config()
	if u := strings.TrimRight(cfg.GetString("bakery.dashboard.url"), "/"); u != "" {
		return u
	}
	if d := cfg.GetString("bakery.dashboard.domain"); d != "" {
		return "https://" + d
	}
	return "http://localhost:4930"
}

// Start runs the dispatcher until ctx ends, resuming Deliveries a previous
// process left pending.
func Start(ctx context.Context) {
	subscribe()
	go svc().Run(ctx)
}
