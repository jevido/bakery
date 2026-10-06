// Package guilds is what other contexts, the router and bootstrap may use
// from the guilds context: the Auth, Deploy, Admin, Secrets and Owns
// middlewares, Current, RoleOf and CanSeeSecrets, the routes, the
// InvitationCreated event, and Boot. Nothing else in contexts/guilds is for
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

var service = app.NewService(infra.Guilds{}, infra.Memberships{}, infra.Invitations{}, members{})

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

// RoleOf is the Role the request acts with in the Current guild: "viewer",
// "member" or "admin".
func RoleOf(ctx contractshttp.Context) string { return string(guildshttp.RoleOf(ctx)) }

// Routes registers `GET /api/me`, an Invitation link (open), the Members
// and Invitations of the Current guild, and identity's API token routes
// behind guilds' middlewares.
func Routes(r route.Router) {
	c := guildshttp.NewController(service)
	c.Invited = publishInvitation
	r.Get("/api/invitations/by-token/{token}", c.InvitationByToken)
	r.Post("/api/invitations/by-token/{token}/accept", c.AcceptInvitation)
	r.Middleware(guildshttp.Auth{Service: service, Guildless: true}).Get("/api/me", c.Me)
	r.Middleware(guildshttp.Auth{Service: service, SelfService: true}).Group(identity.APITokenRoutes)
	r.Middleware(Auth, Admin).Group(func(r route.Router) {
		r.Get("/api/members", c.Members)
		r.Patch("/api/members/{id}", c.ChangeRole)
		r.Delete("/api/members/{id}", c.RemoveMember)
		r.Delete("/api/members/{id}/two-factor", c.ResetTwoFactor)
		r.Get("/api/invitations", c.Invitations)
		r.Post("/api/invitations", c.Invite)
		r.Delete("/api/invitations/{id}", c.RevokeInvitation)
	})
}

// Boot subscribes guilds to what identity announces: Setup's Instance admin
// gets the first Guild, "Default", with an admin Membership.
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
		Email: inv.Email, Role: string(inv.Role), GuildID: guild.ID, Guild: guild.Name,
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
