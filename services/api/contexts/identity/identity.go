// Package identity is what other contexts and the router may use from the
// identity context: its routes, the Auth, Admin and Secrets middlewares and
// CanSeeSecrets. Nothing else in
// contexts/identity is for outside use.
package identity

import (
	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/identity/app"
	identityhttp "github.com/jevido/bakery/services/api/contexts/identity/http"
	"github.com/jevido/bakery/services/api/contexts/identity/infra"
)

var service = app.NewService(infra.Members{}, infra.Hasher{})

// Auth refuses requests that come from no Member (401), and anything but
// reading from a viewer (403).
var Auth contractshttp.Middleware = identityhttp.Auth{Service: service}

// Admin, after Auth, lets only admins and the Owner through (403).
var Admin contractshttp.Middleware = identityhttp.Admin{}

// Secrets, after Auth, keeps viewers away from routes that return Secrets
// (403).
var Secrets contractshttp.Middleware = identityhttp.Secrets{}

// CanSeeSecrets reports whether the request may be answered with Secrets;
// for a response that mixes Secrets with what a viewer may see.
func CanSeeSecrets(ctx contractshttp.Context) bool {
	return identityhttp.RoleOf(ctx).CanSeeSecrets()
}

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
