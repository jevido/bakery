// Package guilds is what other contexts, the router and bootstrap may use
// from the guilds context: the Auth, Deploy, Admin and Secrets middlewares,
// Current, RoleOf and CanSeeSecrets, the routes, and Boot. Nothing else in
// contexts/guilds is for outside use.
package guilds

import (
	"context"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	guildshttp "github.com/jevido/bakery/services/api/contexts/guilds/http"
	"github.com/jevido/bakery/services/api/contexts/guilds/infra"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

var service = app.NewService(infra.Guilds{}, infra.Memberships{}, members{})

// Auth refuses requests that come from no Member (401), from a Member in no
// Guild (403 `you are in no guild`), and anything but reading from a viewer
// of the Current guild (403). An API token acts in the Guild it was made in,
// a Session in the one its `bakery_guild` cookie names (else the Member's
// first).
var Auth contractshttp.Middleware = guildshttp.Auth{Service: service}

// Deploy is Auth for Coolify's deploy actions (deploy, restart, stop,
// start, cancel, rollback): an API token needs the deploy Permission there
// instead of write.
var Deploy contractshttp.Middleware = guildshttp.Auth{Service: service, Deploy: true}

// Admin, after Auth, lets only admins of the Current guild through (403).
var Admin contractshttp.Middleware = guildshttp.Admin{}

// Secrets, after Auth, keeps viewers and API tokens without read:sensitive
// away from routes that return Secrets (403).
var Secrets contractshttp.Middleware = guildshttp.Secrets{}

// CanSeeSecrets reports whether the request may be answered with Secrets;
// for a response that mixes Secrets with what a viewer may see.
func CanSeeSecrets(ctx contractshttp.Context) bool { return guildshttp.CanSeeSecrets(ctx) }

// Current is the id of the Guild the request acts in, after Auth.
func Current(ctx contractshttp.Context) uint64 { return guildshttp.Current(ctx) }

// RoleOf is the Role the request acts with in the Current guild: "viewer",
// "member" or "admin".
func RoleOf(ctx contractshttp.Context) string { return string(guildshttp.RoleOf(ctx)) }

// Routes registers `GET /api/me`, the Members of the Current guild, and
// identity's API token and Invitation routes behind guilds' middlewares.
func Routes(r route.Router) {
	c := guildshttp.NewController(service)
	r.Middleware(guildshttp.Auth{Service: service, Guildless: true}).Get("/api/me", c.Me)
	r.Middleware(guildshttp.Auth{Service: service, SelfService: true}).Group(identity.APITokenRoutes)
	r.Middleware(Auth, Admin).Group(func(r route.Router) {
		r.Get("/api/members", c.Members)
		r.Patch("/api/members/{id}", c.ChangeRole)
		r.Delete("/api/members/{id}", c.RemoveMember)
		r.Delete("/api/members/{id}/two-factor", c.ResetTwoFactor)
		identity.InvitationRoutes(r)
	})
}

// Boot subscribes guilds to what identity announces: Setup's Instance admin
// gets the first Guild, "Default", with an admin Membership, and someone
// who accepted an Invitation a Membership with its Role.
func Boot() {
	identity.OnMemberAdded(func(ctx context.Context, e identity.MemberAdded) error {
		return service.MemberAdded(ctx, e.MemberID, e.InstanceAdmin, e.Role)
	})
}

// members is identity, as guilds' app sees it.
type members struct{}

func (members) IsInstanceAdmin(ctx context.Context, memberID uint64) (bool, error) {
	m, found, err := identity.MemberByID(ctx, memberID)
	return found && m.InstanceAdmin, err
}

func (members) RevokeAPITokens(ctx context.Context, memberID, guildID uint64) error {
	return identity.RevokeAPITokens(ctx, memberID, guildID)
}

func (members) ResetTwoFactor(ctx context.Context, memberID uint64) error {
	return identity.ResetTwoFactor(ctx, memberID)
}
