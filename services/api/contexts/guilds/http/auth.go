// Package http exposes guilds over HTTP: the Auth, Deploy, Admin and Secrets
// middlewares every context's routes sit behind, `GET /api/me` and the
// Members of the Current guild.
package http

import (
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

// GuildCookie names the Current guild of a Session by its id. It grants
// nothing: Auth checks on every request that the Member may act there, and
// falls back to their first Guild when not.
const GuildCookie = "bakery_guild"

type placeKey struct{}

// place is who a request comes from, the Guild it acts in (zero when none)
// and the Role there.
type place struct {
	principal identity.Principal
	guild     domain.Guild
	role      domain.Role
}

// Auth lets a request through only from a Member (identity.Authenticate)
// acting in a Guild: the API token's Guild, or for a Session the Guild in
// GuildCookie when the Member may act there, else their first. The Role is
// their Membership's there, admin for the Instance admin, read on every
// request so a changed Role counts at once. A viewer is refused anything but
// reading, and an API token anything its Permissions do not cover.
type Auth struct {
	Service *app.Service
	// Deploy is for Coolify's deploy actions (deploy, restart, stop, start,
	// cancel, rollback): an API token needs the deploy Permission for them
	// rather than write.
	Deploy bool
	// SelfService is for a Member's own API tokens: every Role may manage
	// them, but only with a Session, so a leaked token cannot mint more.
	SelfService bool
	// Guildless lets a Member who is in no Guild through, with no Current
	// guild (`GET /api/me`).
	Guildless bool
}

func (Auth) Signature() string { return "guilds.auth" }

func (a Auth) Handle(ctx contractshttp.Context) {
	p, ok := identity.Authenticate(ctx)
	if !ok {
		return
	}
	if a.SelfService && p.Token {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "this needs a signed-in session, not an API token").Abort()
		return
	}
	pl, found, err := a.Service.Place(ctx.Context(), p.MemberID, p.InstanceAdmin, p.GuildID, cookieGuild(ctx))
	if err != nil {
		_ = respond.ServerError(ctx, err).Abort()
		return
	}
	switch {
	case !found && p.Token:
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "you are not in this API token's guild").Abort()
		return
	case !found && !a.Guildless:
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "you are in no guild").Abort()
		return
	}
	if !a.SelfService {
		method := ctx.Request().Method()
		if need := required(method, a.Deploy); !p.Allows(need) {
			_ = respond.Error(ctx, contractshttp.StatusForbidden, "Missing required permissions: "+string(need)).Abort()
			return
		}
		if reason := refusal(method, pl.Role); found && reason != "" {
			_ = respond.Error(ctx, contractshttp.StatusForbidden, reason).Abort()
			return
		}
	}
	ctx.WithValue(placeKey{}, place{principal: p, guild: pl.Guild, role: pl.Role})
	if found {
		identity.ActIn(ctx, pl.Guild.ID, string(pl.Role))
	}
	ctx.Request().Next()
}

// cookieGuild is the Guild id GuildCookie names, 0 without a usable one.
func cookieGuild(ctx contractshttp.Context) uint64 {
	id, err := strconv.ParseUint(ctx.Request().Cookie(GuildCookie), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// required is the Permission an API token needs for a request with this
// method: read to read, deploy on a deploy route, write for other changes.
func required(method string, deploy bool) identity.Permission {
	switch {
	case method == contractshttp.MethodGet || method == contractshttp.MethodHead:
		return identity.PermissionRead
	case deploy:
		return identity.PermissionDeploy
	default:
		return identity.PermissionWrite
	}
}

// refusal is why a Role may not make a request with this method, or "".
func refusal(method string, role domain.Role) string {
	if method == contractshttp.MethodGet || method == contractshttp.MethodHead || role.CanWrite() {
		return ""
	}
	return "your role cannot change this"
}

// Admin lets only admins of the Current guild through. It runs after Auth.
type Admin struct{}

func (Admin) Signature() string { return "guilds.admin" }

func (Admin) Handle(ctx contractshttp.Context) {
	if !RoleOf(ctx).IsAdmin() {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "only admins can do this").Abort()
		return
	}
	ctx.Request().Next()
}

// Secrets keeps viewers, and API tokens without read:sensitive, away from
// routes that return Secrets. It runs after Auth.
type Secrets struct{}

func (Secrets) Signature() string { return "guilds.secrets" }

func (Secrets) Handle(ctx contractshttp.Context) {
	p := placeOf(ctx)
	if !p.role.CanSeeSecrets() {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "your role cannot see secrets").Abort()
		return
	}
	if !p.principal.Allows(identity.PermissionReadSensitive) {
		_ = respond.Error(ctx, contractshttp.StatusForbidden, "Missing required permissions: "+string(identity.PermissionReadSensitive)).Abort()
		return
	}
	ctx.Request().Next()
}

func placeOf(ctx contractshttp.Context) place {
	p, _ := ctx.Value(placeKey{}).(place)
	return p
}

// Current is the id of the Guild the request acts in (0 before Auth ran or
// on a Guildless route for someone in none).
func Current(ctx contractshttp.Context) uint64 { return placeOf(ctx).guild.ID }

// RoleOf is the Role the request acts with in the Current guild ("" when
// none).
func RoleOf(ctx contractshttp.Context) domain.Role { return placeOf(ctx).role }

// MemberID is the Member the request comes from.
func MemberID(ctx contractshttp.Context) uint64 { return placeOf(ctx).principal.MemberID }

// CanSeeSecrets reports whether the request may be answered with Secrets:
// its Role may see them and its API token, if any, carries read:sensitive.
func CanSeeSecrets(ctx contractshttp.Context) bool {
	p := placeOf(ctx)
	return p.role.CanSeeSecrets() && p.principal.Allows(identity.PermissionReadSensitive)
}
