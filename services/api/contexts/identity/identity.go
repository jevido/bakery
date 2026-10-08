// Package identity is what other contexts and the router may use from the
// identity context: Authenticate and the Principal it finds, the Members
// guilds lists and the new ones it creates for Invitations, SignIn, the
// routes, the SetUp event and the artisan Commands. Nothing else in contexts/identity is for
// outside use.
package identity

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

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
	s := app.NewService(infra.Members{}, infra.APITokens{}, infra.DesktopSignIns{}, infra.Hasher{})
	s.SetUpDone = publishSetUp
	return s
}()

var controller = identityhttp.NewController(service)

// Permission is a Token permission: what an API token may do, Coolify's
// abilities.
type Permission string

const (
	PermissionRoot          Permission = "root"
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
	// GuildID is the Guild the API token was made in; 0 for a Session and
	// a Desktop key.
	GuildID uint64
	// DesktopID is the Desktop whose Desktop key made the request, 0
	// otherwise. A Desktop key acts as its Member in any of their Guilds,
	// uncapped by Token permissions.
	DesktopID   uint64
	permissions []domain.Permission
}

// Allows reports whether the request may do what perm covers as far as its
// API token goes: a Session always may, a token when it carries perm or
// root. The Member's Permissions in the Guild limit both further; that is
// guilds' check.
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

// Authenticate finds the Principal of a request, by Desktop key or API
// token in the Authorization header or the Session cookie, and answers 401 itself (and
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
	if p.Desktop != nil {
		out.DesktopID = p.Desktop.ID
	}
	return out, true
}

// ActIn tells identity's routes served inside a Guild (API tokens) which
// Guild the request acts in and the wire keys of the Member's Permissions
// there, administrator spelled out as every one.
func ActIn(ctx contractshttp.Context, guildID uint64, permissions []string) {
	identityhttp.ActIn(ctx, guildID, domain.MemberPermissions(permissions))
}

// Member is a person who may sign in, as other contexts see them.
type Member struct {
	ID            uint64
	Name          string
	Email         string
	TwoFactor     bool
	InstanceAdmin bool
}

// DesktopOnly lets through only a request with a Desktop key, answering
// 401 without one and 403 for a Session or an API token; DesktopOf reads it.
var DesktopOnly contractshttp.Middleware = identityhttp.DesktopOnly{Service: service}

// DesktopOf is the Member and the Desktop whose Desktop key made a request
// DesktopOnly let through.
func DesktopOf(ctx contractshttp.Context) (memberID, desktopID uint64, ok bool) {
	return identityhttp.DesktopOf(ctx)
}

// SeeDesktop records that the Desktop is still connected, for a stream it
// holds open past the one request that counted.
func SeeDesktop(ctx context.Context, desktopID uint64) error {
	return service.SeeDesktop(ctx, desktopID)
}

// DesktopNames names the Desktops among ids, signed out ones included, for
// the Runs they ran.
func DesktopNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	return service.DesktopNames(ctx, ids)
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

// MemberByEmail is the Member with this email (compared as identity stores
// emails); false when there is none.
func MemberByEmail(ctx context.Context, email string) (Member, bool, error) {
	m, found, err := service.MemberByEmail(ctx, email)
	if err != nil || !found {
		return Member{}, false, err
	}
	return MemberByID(ctx, m.ID)
}

// The reasons CreateMember refuses a new Member.
var (
	ErrInvalidName      = domain.ErrInvalidName
	ErrInvalidEmail     = domain.ErrInvalidEmail
	ErrPasswordTooShort = domain.ErrPasswordTooShort
	ErrEmailTaken       = app.ErrEmailTaken
)

// CreateMember stores a new Member (not the Instance admin) with the name
// and password they picked, for someone accepting an Invitation; the
// Membership is the caller's.
func CreateMember(ctx context.Context, name, email, password string) (Member, error) {
	m, err := service.CreateMember(ctx, name, email, password)
	if err != nil {
		return Member{}, err
	}
	return Member{ID: m.ID, Name: m.Name, Email: m.Email}, nil
}

// SignIn starts a Session for the Member: the response carries its cookie.
func SignIn(ctx contractshttp.Context, memberID uint64) error {
	return identityhttp.SignIn(ctx, memberID)
}

// SessionMember is the Member whose Session the request carries, answering
// nothing itself; false without a Session that still counts. API tokens
// are not looked at.
func SessionMember(ctx contractshttp.Context) (Member, bool, error) {
	m, found, err := identityhttp.SessionMember(service, ctx)
	if err != nil || !found {
		return Member{}, false, err
	}
	return Member{ID: m.ID, Name: m.Name, Email: m.Email, TwoFactor: m.TwoFactor.On(), InstanceAdmin: m.InstanceAdmin}, true, nil
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

// Routes registers setup, login, logout, the Desktop sign-in (all open), a
// Member's own Desktops (a Session or a Desktop key) and their Profile and
// two-factor (a Session only).
func Routes(r route.Router) {
	c := controller
	r.Get("/api/setup", c.SetupStatus)
	r.Post("/api/setup", c.Setup)
	r.Post("/api/login", c.Login)
	r.Post("/api/login/two-factor", c.LoginTwoFactor)
	r.Post("/api/logout", c.Logout)
	// The Desktop app's sign-in: open, guarded by the secret the app holds;
	// approve reads the Session itself.
	r.Post("/api/desktop-sign-ins", c.StartDesktopSignIn)
	r.Get("/api/desktop-sign-ins/{id}", c.DescribeDesktopSignIn)
	r.Post("/api/desktop-sign-ins/{id}/approve", c.ApproveDesktopSignIn)
	r.Post("/api/desktop-sign-ins/{id}/cancel", c.CancelDesktopSignIn)
	// A Member's own Desktops: with a Session or a Desktop key.
	r.Middleware(identityhttp.DesktopAuth{Service: service}).Group(func(r route.Router) {
		r.Get("/api/desktops", c.Desktops)
		r.Post("/api/desktops/current/sign-out", c.SignOutCurrentDesktop)
		r.Delete("/api/desktops/{id}", c.SignOutDesktop)
	})
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

// Commands are identity's artisan commands.
func Commands() []contractsconsole.Command {
	return []contractsconsole.Command{identityconsole.ResetTwoFactor{Service: service}}
}

var (
	setUpMu sync.Mutex
	onSetUp func(ctx context.Context, instanceAdminID uint64) error
)

// OnSetUp registers the one subscriber of Setup having stored the Instance
// admin (guilds: the first Guild). Its error fails the request; the
// Instance admin stays.
func OnSetUp(f func(ctx context.Context, instanceAdminID uint64) error) {
	setUpMu.Lock()
	defer setUpMu.Unlock()
	onSetUp = f
}

func publishSetUp(ctx context.Context, instanceAdminID uint64) error {
	setUpMu.Lock()
	f := onSetUp
	setUpMu.Unlock()
	if f == nil {
		return nil
	}
	return f(ctx, instanceAdminID)
}

func first(instanceAdmin bool) int {
	if instanceAdmin {
		return 1
	}
	return 0
}
