// Package notifications is what the router and the boot code may use from
// the notifications context: its routes. Nothing else in
// contexts/notifications is for outside use.
package notifications

import (
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
		service = app.NewService(infra.Store{})
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
	})
}
