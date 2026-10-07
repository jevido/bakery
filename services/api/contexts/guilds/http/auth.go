// Package http exposes guilds over HTTP: the Auth and Can middlewares every
// context's routes sit behind, `GET /api/me`, and the
// Members and Invitations of the Current guild.
package http

import (
	"context"
	"strconv"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
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
// and the Permissions there, already capped by an API token's Token
// permissions (effective).
type place struct {
	principal   identity.Principal
	guild       domain.Guild
	permissions domain.Permissions
	// held are the Member's own Permissions, before an API token's cap.
	held domain.Permissions
	// at is where the Member acts, for resolving a Project's overrides.
	at app.Place
	// project is the Project the route is in (InProject), 0 for none;
	// permissions and held are then resolved there.
	project uint64
}

// Auth lets a request through only from a Member (identity.Authenticate)
// acting in a Guild: the API token's Guild, or for a Session the Guild in
// GuildCookie when the Member may act there, else their first. The
// Permissions are their Membership's there, every one for the Instance
// admin, read on every request so a changed Role counts at once, and capped
// by an API token's Token permissions (effective). Reading needs
// view_resources; an API token is refused anything its Token permissions do
// not cover by method. Every change names its Permission with Can.
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
	}
	perms := effective(p, pl.Permissions)
	if found && !a.SelfService && !a.Guildless && reads(ctx.Request().Method()) && !perms.Has(domain.PermissionViewResources) {
		_ = refuse(ctx, p, pl.Permissions, domain.PermissionViewResources)
		return
	}
	ctx.WithValue(placeKey{}, place{principal: p, guild: pl.Guild, permissions: perms, held: pl.Permissions, at: pl})
	if found {
		identity.ActIn(ctx, pl.Guild.ID, pl.Permissions.Expand().Keys())
	}
	ctx.Request().Next()
}

// SetCurrent makes the Guild the Current guild of the browser's Session from
// the next request on. Like the Session cookie it is host-only, for the API's
// paths and SameSite=Strict.
func SetCurrent(ctx contractshttp.Context, guildID uint64) {
	guildCookie(ctx, strconv.FormatUint(guildID, 10), int((400 * 24 * time.Hour).Seconds()))
}

// forgetCurrent removes GuildCookie, so the Session acts in the Member's
// first Guild again. WithoutCookie would miss it: it clears the cookie for
// no path, and this one lives on /api.
func forgetCurrent(ctx contractshttp.Context) {
	guildCookie(ctx, "", -1)
}

func guildCookie(ctx contractshttp.Context, value string, maxAge int) {
	ctx.Response().Cookie(contractshttp.Cookie{
		Name:     GuildCookie,
		Value:    value,
		Path:     "/api",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   facades.Config().GetString("app.env") == "production",
		SameSite: "strict",
	})
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
	case reads(method):
		return identity.PermissionRead
	case deploy:
		return identity.PermissionDeploy
	default:
		return identity.PermissionWrite
	}
}

func reads(method string) bool {
	return method == contractshttp.MethodGet || method == contractshttp.MethodHead
}

// changes are the Permissions that change something, which an API token
// keeps only with write or root.
var changes = domain.AllPermissions.Without(domain.Of(domain.PermissionAdministrator, domain.PermissionViewResources, domain.PermissionSeeSecrets, domain.PermissionDeploy))

// effective is what a request may do with the Member's Permissions: all of
// them with a Session; with an API token only what its Token permissions
// cover: see_secrets with read:sensitive, deploy with deploy, the other
// changes with write, administrator only with root.
func effective(p identity.Principal, perms domain.Permissions) domain.Permissions {
	if !p.Token {
		return perms
	}
	out := perms.Expand()
	if !p.Allows(identity.PermissionRoot) {
		out = out.Without(domain.Of(domain.PermissionAdministrator))
	}
	if !p.Allows(identity.PermissionReadSensitive) {
		out = out.Without(domain.Of(domain.PermissionSeeSecrets))
	}
	if !p.Allows(identity.PermissionDeploy) {
		out = out.Without(domain.Of(domain.PermissionDeploy))
	}
	if !p.Allows(identity.PermissionWrite) {
		out = out.Without(changes)
	}
	return out
}

// refuse answers 403 for a request without need: in Coolify's words when
// the Member holds it but their API token does not carry it, else naming the
// Permission.
func refuse(ctx contractshttp.Context, p identity.Principal, held domain.Permissions, need domain.Permission) error {
	if p.Token && held.Has(need) {
		return respond.Error(ctx, contractshttp.StatusForbidden, "Missing required permissions: "+string(tokenPermission(need))).Abort()
	}
	return respond.Error(ctx, contractshttp.StatusForbidden, "you need the "+need.Name()+" permission").Abort()
}

// tokenPermission is the Token permission that lets an API token use need.
func tokenPermission(need domain.Permission) identity.Permission {
	switch need {
	case domain.PermissionAdministrator:
		return identity.PermissionRoot
	case domain.PermissionViewResources:
		return identity.PermissionRead
	case domain.PermissionSeeSecrets:
		return identity.PermissionReadSensitive
	case domain.PermissionDeploy:
		return identity.PermissionDeploy
	}
	return identity.PermissionWrite
}

// Can lets only requests that may use Permission in the Current guild
// through (403), or in the request's Project after InProject. It runs
// after Auth.
type Can struct {
	Permission domain.Permission
}

func (c Can) Signature() string { return "guilds.can." + c.Permission.Key() }

func (c Can) Handle(ctx contractshttp.Context) {
	p := placeOf(ctx)
	if !p.permissions.Has(c.Permission) {
		_ = refuse(ctx, p.principal, p.held, c.Permission)
		return
	}
	ctx.Request().Next()
}

// Allows reports whether the request may use perm in the Current guild,
// for a handler whose answer differs by Permission.
func Allows(ctx contractshttp.Context, perm domain.Permission) bool {
	return placeOf(ctx).permissions.Has(perm)
}

func placeOf(ctx contractshttp.Context) place {
	p, _ := ctx.Value(placeKey{}).(place)
	return p
}

// Current is the id of the Guild the request acts in (0 before Auth ran or
// on a Guildless route for someone in none).
func Current(ctx contractshttp.Context) uint64 { return placeOf(ctx).guild.ID }

// PermissionsOf is what the request may do in the Current guild (none
// before Auth ran or without a Current guild).
func PermissionsOf(ctx contractshttp.Context) domain.Permissions { return placeOf(ctx).permissions }

// wireRole is the former role ("admin", "member" or "viewer") these
// Permissions read as where the wire still has one, until the dashboard
// asks for Permissions itself.
func wireRole(perms domain.Permissions) string {
	switch {
	case perms.Has(domain.PermissionAdministrator):
		return "admin"
	case perms.Has(domain.PermissionManageApplications):
		return "member"
	}
	return "viewer"
}

// InstanceAdmin reports whether the request comes from the Instance admin.
func InstanceAdmin(ctx contractshttp.Context) bool { return placeOf(ctx).principal.InstanceAdmin }

// MemberID is the Member the request comes from.
func MemberID(ctx contractshttp.Context) uint64 { return placeOf(ctx).principal.MemberID }

// Owns answers 404 when the route's {id} names something that is not in the
// Current guild; Belongs is asked by the context that owns it. A malformed
// {id} is left to the handler. It runs after Auth.
type Owns struct {
	// Name tells the middlewares apart, e.g. "application".
	Name    string
	Belongs func(ctx context.Context, id, guildID uint64) (bool, error)
}

func (o Owns) Signature() string { return "guilds.owns." + o.Name }

func (o Owns) Handle(ctx contractshttp.Context) {
	id, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
	if err != nil {
		ctx.Request().Next()
		return
	}
	ok, err := o.Belongs(ctx.Context(), id, Current(ctx))
	if err != nil {
		_ = respond.ServerError(ctx, err).Abort()
		return
	}
	if !ok {
		_ = respond.Error(ctx, contractshttp.StatusNotFound, "not found").Abort()
		return
	}
	ctx.Request().Next()
}
