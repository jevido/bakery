// Package identity is what other contexts and the router may use from the
// identity context: Authenticate and the Principal it finds, the Members
// guilds lists, the routes, the MemberAdded and InvitationCreated events
// and the artisan Commands. Nothing else in contexts/identity is for
// outside use.
package identity

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	contractsconsole "github.com/goravel/framework/contracts/console"
	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/identity/app"
	identityconsole "github.com/jevido/bakery/services/api/contexts/identity/console"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
	identityhttp "github.com/jevido/bakery/services/api/contexts/identity/http"
	"github.com/jevido/bakery/services/api/contexts/identity/infra"
)

var service = func() *app.Service {
	s := app.NewService(infra.Members{}, infra.Invitations{}, infra.APITokens{}, infra.Hasher{})
	s.MemberAdded = publishMemberAdded
	return s
}()

var controller = func() *identityhttp.Controller {
	c := identityhttp.NewController(service)
	c.Invited = publishInvitation
	return c
}()

// Permission is what an API token may do: Coolify's abilities.
type Permission string

const (
	PermissionRead          Permission = "read"
	PermissionReadSensitive Permission = "read:sensitive"
	PermissionWrite         Permission = "write"
	PermissionDeploy        Permission = "deploy"
)

// Principal is who a request comes from.
type Principal struct {
	MemberID      uint64
	InstanceAdmin bool
	// Token is true for a request made with an API token, false for a
	// Session.
	Token bool
	// GuildID is the Guild the API token was made in; 0 for a Session.
	GuildID     uint64
	permissions []domain.Permission
}

// Allows reports whether the request may do what perm covers as far as its
// API token goes: a Session always may, a token when it carries perm or
// root. The Role in the Guild limits both further; that is guilds' check.
func (p Principal) Allows(perm Permission) bool {
	if !p.Token {
		return true
	}
	return domain.APIToken{Permissions: p.permissions}.Allows(domain.Permission(perm))
}

// TokenPrincipal is a Principal of an API token with these Permissions, for
// other contexts' tests.
func TokenPrincipal(memberID, guildID uint64, permissions ...Permission) Principal {
	p := Principal{MemberID: memberID, Token: true, GuildID: guildID}
	for _, perm := range permissions {
		p.permissions = append(p.permissions, domain.Permission(perm))
	}
	return p
}

// Authenticate finds the Principal of a request, by API token in the
// Authorization header or the Session cookie, and answers 401 itself (and
// false) when there is none.
func Authenticate(ctx contractshttp.Context) (Principal, bool) {
	p, ok := identityhttp.Authenticate(service, ctx)
	if !ok {
		return Principal{}, false
	}
	out := Principal{MemberID: p.MemberID, InstanceAdmin: p.InstanceAdmin}
	if p.Token != nil {
		out.Token = true
		out.GuildID = p.Token.GuildID
		out.permissions = p.Token.Permissions
	}
	return out, true
}

// ActIn tells identity's routes served inside a Guild (API tokens,
// Invitations) which Guild the request acts in and the Member's Role
// there ("viewer", "member" or "admin").
func ActIn(ctx contractshttp.Context, guildID uint64, role string) {
	identityhttp.ActIn(ctx, guildID, domain.Role(role))
}

// Member is a person who may sign in, as other contexts see them.
type Member struct {
	ID            uint64
	Name          string
	Email         string
	TwoFactor     bool
	InstanceAdmin bool
}

// Members lists the Members with these ids, the Instance admin first, then
// by name; ids of removed Members are left out.
func Members(ctx context.Context, ids []uint64) ([]Member, error) {
	ms, err := service.MembersByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(ms))
	for i, m := range ms {
		out[i] = Member{ID: m.ID, Name: m.Name, Email: m.Email, TwoFactor: m.TwoFactor.On(), InstanceAdmin: m.InstanceAdmin}
	}
	slices.SortFunc(out, func(a, b Member) int {
		return cmp.Or(
			cmp.Compare(first(b.InstanceAdmin), first(a.InstanceAdmin)),
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID, b.ID),
		)
	})
	return out, nil
}

// MemberByID is one Member; false when there is none.
func MemberByID(ctx context.Context, id uint64) (Member, bool, error) {
	ms, err := Members(ctx, []uint64{id})
	if err != nil || len(ms) == 0 {
		return Member{}, false, err
	}
	return ms[0], true, nil
}

// ErrTwoFactorOff is ResetTwoFactor's answer for a Member whose two-factor
// is off.
var ErrTwoFactorOff = app.ErrTwoFactorOff

// ErrMemberNotFound is the answer for an id with no Member.
var ErrMemberNotFound = app.ErrMemberNotFound

// ResetTwoFactor switches a locked-out Member's two-factor off and ends their
// Sessions. Who may do this is the caller's to check.
func ResetTwoFactor(ctx context.Context, memberID uint64) error {
	return service.ResetTwoFactor(ctx, memberID)
}

// RevokeAPITokens deletes every API token the Member made in the Guild, for
// when they leave it.
func RevokeAPITokens(ctx context.Context, memberID, guildID uint64) error {
	return service.RevokeAPITokensIn(ctx, memberID, guildID)
}

// Routes registers setup, login, logout, an Invitation link (all open) and
// a Member's own Profile and two-factor (a Session only).
func Routes(r route.Router) {
	c := controller
	r.Get("/api/setup", c.SetupStatus)
	r.Post("/api/setup", c.Setup)
	r.Post("/api/login", c.Login)
	r.Post("/api/login/two-factor", c.LoginTwoFactor)
	r.Post("/api/logout", c.Logout)
	r.Get("/api/invitations/by-token/{token}", c.InvitationByToken)
	r.Post("/api/invitations/by-token/{token}/accept", c.AcceptInvitation)
	r.Middleware(identityhttp.Auth{Service: service}).Group(func(r route.Router) {
		r.Patch("/api/me", c.ChangeName)
		r.Post("/api/me/password", c.ChangePassword)
		r.Post("/api/me/sign-out-others", c.SignOutOtherSessions)
		r.Get("/api/me/two-factor", c.TwoFactor)
		r.Post("/api/me/two-factor", c.StartTwoFactor)
		r.Post("/api/me/two-factor/confirm", c.ConfirmTwoFactor)
		r.Post("/api/me/two-factor/recovery-codes", c.RegenerateRecoveryCodes)
		r.Delete("/api/me/two-factor", c.DisableTwoFactor)
	})
}

// APITokenRoutes registers a Member's own API tokens in the Current guild.
// The caller wraps them in a middleware that lets only a Session through
// and calls ActIn (guilds).
func APITokenRoutes(r route.Router) {
	c := controller
	r.Get("/api/api-tokens", c.APITokens)
	r.Get("/api/api-tokens/permissions", c.APITokenPermissions)
	r.Post("/api/api-tokens", c.CreateAPIToken)
	r.Delete("/api/api-tokens/{id}", c.RevokeAPIToken)
}

// InvitationRoutes registers listing, making and revoking Invitations. The
// caller wraps them in a middleware that lets only admins through (guilds).
func InvitationRoutes(r route.Router) {
	c := controller
	r.Get("/api/invitations", c.Invitations)
	r.Post("/api/invitations", c.Invite)
	r.Delete("/api/invitations/{id}", c.RevokeInvitation)
}

// Commands are identity's artisan commands.
func Commands() []contractsconsole.Command {
	return []contractsconsole.Command{identityconsole.ResetTwoFactor{Service: service}}
}

// MemberAdded is a Member just stored: the Instance admin by Setup, or
// someone who accepted an Invitation.
type MemberAdded struct {
	MemberID      uint64
	InstanceAdmin bool
	// Role is the Invitation's ("admin", "member" or "viewer"); admin for
	// the Instance admin.
	Role string
}

var (
	addedMu sync.Mutex
	onAdded func(ctx context.Context, e MemberAdded) error
)

// OnMemberAdded registers the one subscriber of MemberAdded (guilds: the
// first Guild after Setup, a Membership after an accepted Invitation). Its
// error fails the request; the Member stays.
func OnMemberAdded(f func(ctx context.Context, e MemberAdded) error) {
	addedMu.Lock()
	defer addedMu.Unlock()
	onAdded = f
}

func publishMemberAdded(ctx context.Context, e app.MemberAdded) error {
	addedMu.Lock()
	f := onAdded
	addedMu.Unlock()
	if f == nil {
		return nil
	}
	return f(ctx, MemberAdded{MemberID: e.MemberID, InstanceAdmin: e.InstanceAdmin, Role: string(e.Role)})
}

// InvitationCreated is an Invitation just made, with its link: the only
// moment the link is known.
type InvitationCreated struct {
	Email string
	// Role is "admin", "member" or "viewer".
	Role      string
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

func publishInvitation(ctx context.Context, inv domain.Invitation, invitedBy, link string) (bool, error) {
	invitedMu.Lock()
	f := onInvited
	invitedMu.Unlock()
	if f == nil {
		return false, nil
	}
	return f(ctx, InvitationCreated{Email: inv.Email, Role: string(inv.Role), InvitedBy: invitedBy, Link: link, ExpiresAt: inv.ExpiresAt})
}

func first(instanceAdmin bool) int {
	if instanceAdmin {
		return 1
	}
	return 0
}
