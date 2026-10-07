// Package guilds is what other contexts, the router and bootstrap may use
// from the guilds context: the Auth, Deploy, Can and Owns middlewares,
// Current and Allows, IsGuildMaster, the routes, the
// InvitationCreated event, the OnGuildDeleting check, and Boot. Nothing else in contexts/guilds is for
// outside use.
package guilds

import (
	"context"
	"errors"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	guildshttp "github.com/jevido/bakery/services/api/contexts/guilds/http"
	"github.com/jevido/bakery/services/api/contexts/guilds/infra"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

var service = app.NewService(infra.Guilds{}, infra.Memberships{}, infra.Roles{}, infra.Invitations{}, infra.Offers{}, members{})

// Auth refuses requests that come from no Member (401), from a Member in no
// Guild (403 `you are in no guild`), reading without view_resources in the
// Current guild (403), and an API token anything its Token permissions do
// not cover by method (403). An API token acts in the Guild it was made in,
// a Session in the one its `bakery_guild` cookie names (else the Member's
// first). Changes are refused only by Can, so every change route names its
// Permission.
var Auth contractshttp.Middleware = guildshttp.Auth{Service: service}

// Deploy is Auth for Coolify's deploy actions (deploy, restart, stop,
// start, cancel, rollback): an API token needs the deploy Token permission
// there instead of write. The deploy Permission is Can's.
var Deploy contractshttp.Middleware = guildshttp.Auth{Service: service, Deploy: true}

// Can, after Auth, lets only requests that may use the Permission with
// this wire key (the glossary's list, e.g. "manage_servers") in the Current
// guild through (403 `you need the <Name> permission`). An unknown key
// panics, at boot where routes are registered.
func Can(permission string) contractshttp.Middleware {
	return guildshttp.Can{Permission: mustPermission(permission)}
}

// Allows reports, after Auth, whether the request may use the Permission
// with this wire key in the Current guild, an API token's cap included; for
// a handler whose answer differs by Permission (e.g. Secrets in a response
// a viewer may also read).
func Allows(ctx contractshttp.Context, permission string) bool {
	return guildshttp.Allows(ctx, mustPermission(permission))
}

func mustPermission(key string) domain.Permission {
	p, err := domain.ParsePermission(key)
	if err != nil {
		panic("guilds: unknown permission " + key)
	}
	return p
}

// Owns, after Auth, answers 404 for a route whose {id} names something
// outside the Current guild. belongs is the owning context's check, e.g.
// projects.ApplicationInGuild; name tells the middlewares apart.
func Owns(name string, belongs func(ctx context.Context, id, guildID uint64) (bool, error)) contractshttp.Middleware {
	return guildshttp.Owns{Name: name, Belongs: belongs}
}

// Current is the id of the Guild the request acts in, after Auth.
func Current(ctx contractshttp.Context) uint64 { return guildshttp.Current(ctx) }

// InstanceAdmin reports, after Auth, whether the request comes from the
// Instance admin, who runs the installation (the Local server among it).
func InstanceAdmin(ctx contractshttp.Context) bool { return guildshttp.InstanceAdmin(ctx) }

// Routes registers `GET /api/me`, an Invitation link (open), the Guilds
// (list, create, switch, and the Current guild's General and deletion),
// the Members and Invitations of the Current guild, and identity's API token routes
// behind guilds' middlewares.
func Routes(r route.Router) {
	c := guildshttp.NewController(service)
	c.Invited = publishInvitation
	r.Get("/api/invitations/by-token/{token}", c.InvitationByToken)
	r.Post("/api/invitations/by-token/{token}/accept", c.AcceptInvitation)
	r.Middleware(guildshttp.Auth{Service: service, Guildless: true}).Get("/api/me", c.Me)
	r.Middleware(guildshttp.Auth{Service: service, SelfService: true}).Group(identity.APITokenRoutes)
	// Listing, creating and switching Guilds work in no Guild and for every
	// Role, but only with a Session: an API token acts in one Guild.
	r.Middleware(guildshttp.Auth{Service: service, Guildless: true, SelfService: true}).Group(func(r route.Router) {
		r.Get("/api/guilds", c.Guilds)
		r.Post("/api/guilds", c.CreateGuild)
		r.Post("/api/guilds/{id}/switch", c.SwitchGuild)
		// The offered Member answers a Transfer offer of any Guild they are
		// in, whichever is Current.
		r.Post("/api/guild-master-offers/{id}/accept", c.AcceptOffer)
		r.Post("/api/guild-master-offers/{id}/decline", c.DeclineOffer)
	})
	// Only the Guild Master offers the Guild Master (the service checks),
	// and only with a Session.
	r.Middleware(guildshttp.Auth{Service: service, SelfService: true}).Group(func(r route.Router) {
		r.Post("/api/guilds/current/guild-master-offer", c.OfferGuildMaster)
		r.Delete("/api/guilds/current/guild-master-offer", c.WithdrawOffer)
	})
	// Every Member reads their Guild's General page and Members, as in
	// Coolify.
	r.Middleware(Auth).Get("/api/guilds/current", c.CurrentGuild)
	r.Middleware(Auth).Get("/api/members", c.Members)
	r.Middleware(Auth, Can("manage_guild")).Patch("/api/guilds/current", c.UpdateCurrentGuild)
	r.Middleware(Auth, Can("administrator")).Delete("/api/guilds/current", c.DeleteCurrentGuild)
	r.Middleware(Auth, Can("manage_members")).Group(func(r route.Router) {
		r.Patch("/api/members/{id}", c.ChangeRole)
		r.Delete("/api/members/{id}", c.RemoveMember)
		r.Delete("/api/members/{id}/two-factor", c.ResetTwoFactor)
		r.Get("/api/invitations", c.Invitations)
		r.Post("/api/invitations", c.Invite)
		r.Delete("/api/invitations/{id}", c.RevokeInvitation)
	})
}

// OnGuildDeleting registers a check asked before a Guild is deleted: a
// context that keeps something of kind (e.g. "projects") in the Guild
// answers true while it does, and the deletion is refused (409) naming kind.
func OnGuildDeleting(kind string, inUse func(ctx context.Context, guildID uint64) (bool, error)) {
	service.OnGuildDeleting(kind, inUse)
}

// IsGuildMaster reports whether the Member is the Guild Master of any
// Guild. Such a Member cannot leave that Guild or have their account
// deleted; whatever deletes an account or leaves a Guild asks this first.
func IsGuildMaster(ctx context.Context, memberID uint64) (bool, error) {
	return service.IsGuildMaster(ctx, memberID)
}

// Boot subscribes guilds to what identity announces: Setup's Instance admin
// gets the first Guild, "Default", holding its Admin Role.
func Boot() {
	identity.OnSetUp(service.MakeFirstGuild)
}

// InvitationCreated is an Invitation into a Guild just made, with its link:
// the only moment the link is known.
type InvitationCreated struct {
	Email string
	// Role is "admin", "member" or "viewer".
	Role      string
	GuildID   uint64
	Guild     string
	InvitedBy string
	Link      string
	ExpiresAt time.Time
}

var (
	invitedMu sync.Mutex
	onInvited func(ctx context.Context, e InvitationCreated) (emailed bool, err error)
)

// OnInvitationCreated registers the one subscriber of InvitationCreated.
// It is called while the invite request waits, and answers whether it
// emailed the link (false without an error: there is no way to email).
func OnInvitationCreated(f func(ctx context.Context, e InvitationCreated) (emailed bool, err error)) {
	invitedMu.Lock()
	defer invitedMu.Unlock()
	onInvited = f
}

func publishInvitation(ctx context.Context, inv domain.Invitation, guild domain.Guild, invitedBy, link string) (bool, error) {
	invitedMu.Lock()
	f := onInvited
	invitedMu.Unlock()
	if f == nil {
		return false, nil
	}
	return f(ctx, InvitationCreated{
		Email: inv.Email, Role: inv.Role, GuildID: guild.ID, Guild: guild.Name,
		InvitedBy: invitedBy, Link: link, ExpiresAt: inv.ExpiresAt,
	})
}

// members is identity, as guilds' app sees it.
type members struct{}

func (members) IsInstanceAdmin(ctx context.Context, memberID uint64) (bool, error) {
	m, found, err := identity.MemberByID(ctx, memberID)
	return found && m.InstanceAdmin, err
}

func (members) MemberByEmail(ctx context.Context, email string) (uint64, bool, error) {
	m, found, err := identity.MemberByEmail(ctx, email)
	return m.ID, found, err
}

func (members) CreateMember(ctx context.Context, name, email, password string) (uint64, error) {
	m, err := identity.CreateMember(ctx, name, email, password)
	if errors.Is(err, identity.ErrEmailTaken) {
		return 0, app.ErrMemberExists
	}
	return m.ID, err
}

func (members) RevokeAPITokens(ctx context.Context, memberID, guildID uint64) error {
	return identity.RevokeAPITokens(ctx, memberID, guildID)
}

func (members) ResetTwoFactor(ctx context.Context, memberID uint64) error {
	return identity.ResetTwoFactor(ctx, memberID)
}
