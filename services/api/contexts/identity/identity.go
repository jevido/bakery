// Package identity is what other contexts and the router may use from the
// identity context: its routes, the Auth, Admin and Secrets middlewares,
// CanSeeSecrets and the InvitationCreated event. Nothing else in
// contexts/identity is for outside use.
package identity

import (
	"context"
	"sync"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
	identityhttp "github.com/jevido/bakery/services/api/contexts/identity/http"
	"github.com/jevido/bakery/services/api/contexts/identity/infra"
)

var service = app.NewService(infra.Members{}, infra.Invitations{}, infra.APITokens{}, infra.Hasher{})

// Auth refuses requests that come from no Member (401), by Session cookie
// or `Authorization: Bearer <API token>`, and anything but reading from a
// viewer (403).
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

// Routes registers setup, login, logout, me, Members and Invitations.
// Setup, login and an Invitation link are open; the rest needs a Member, and
// managing Members an admin.
func Routes(r route.Router) {
	c := identityhttp.NewController(service)
	c.Invited = publishInvitation
	r.Get("/api/setup", c.SetupStatus)
	r.Post("/api/setup", c.Setup)
	r.Post("/api/login", c.Login)
	r.Post("/api/logout", c.Logout)
	r.Middleware(Auth).Get("/api/me", c.Me)
	r.Get("/api/invitations/by-token/{token}", c.InvitationByToken)
	r.Post("/api/invitations/by-token/{token}/accept", c.AcceptInvitation)
	// Every Role manages its own API tokens and two-factor, with a Session
	// only.
	r.Middleware(identityhttp.Auth{Service: service, SelfService: true}).Group(func(r route.Router) {
		r.Get("/api/api-tokens", c.APITokens)
		r.Post("/api/api-tokens", c.CreateAPIToken)
		r.Delete("/api/api-tokens/{id}", c.RevokeAPIToken)
		r.Get("/api/me/two-factor", c.TwoFactor)
		r.Post("/api/me/two-factor", c.StartTwoFactor)
		r.Post("/api/me/two-factor/confirm", c.ConfirmTwoFactor)
		r.Post("/api/me/two-factor/recovery-codes", c.RegenerateRecoveryCodes)
		r.Delete("/api/me/two-factor", c.DisableTwoFactor)
	})
	r.Middleware(Auth, Admin).Group(func(r route.Router) {
		r.Get("/api/members", c.Members)
		r.Patch("/api/members/{id}", c.ChangeRole)
		r.Delete("/api/members/{id}", c.RemoveMember)
		r.Get("/api/invitations", c.Invitations)
		r.Post("/api/invitations", c.Invite)
		r.Delete("/api/invitations/{id}", c.RevokeInvitation)
	})
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
