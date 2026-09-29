// Package identity is what other contexts and the router may use from the
// identity context: its routes and the Auth middleware. Nothing else in
// contexts/identity is for outside use.
package identity

import (
	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/identity/app"
	identityhttp "github.com/jevido/bakery/services/api/contexts/identity/http"
	"github.com/jevido/bakery/services/api/contexts/identity/infra"
)

var service = app.NewService(infra.Owners{}, infra.Hasher{})

// Auth refuses requests without a valid Session (401).
var Auth contractshttp.Middleware = identityhttp.Auth{}

// Routes registers setup, login, logout and me. Setup and login are open;
// the rest needs a Session.
func Routes(r route.Router) {
	c := identityhttp.NewController(service)
	r.Get("/api/setup", c.SetupStatus)
	r.Post("/api/setup", c.Setup)
	r.Post("/api/login", c.Login)
	r.Post("/api/logout", c.Logout)
	r.Middleware(Auth).Get("/api/me", c.Me)
}
