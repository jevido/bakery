package http

import (
	"context"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/app"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

var (
	viewer = domain.Of(domain.PermissionViewResources)
	member = domain.Of(domain.PermissionViewResources, domain.PermissionSeeSecrets, domain.PermissionDeploy, domain.PermissionManageApplications)
	admin  = domain.Of(domain.PermissionAdministrator)
)

func TestWireRole(t *testing.T) {
	cases := map[string]domain.Permissions{"viewer": viewer, "member": member, "admin": admin}
	for want, perms := range cases {
		if got := wireRole(perms); got != want {
			t.Errorf("%v: %s, want %s", perms.Keys(), got, want)
		}
	}
	if wireRole(0) != "viewer" || wireRole(domain.AllPermissions) != "admin" {
		t.Error("none reads as viewer, all as admin")
	}
}

func TestRequiredPermission(t *testing.T) {
	cases := []struct {
		method string
		deploy bool
		want   identity.Permission
	}{
		{"GET", false, identity.PermissionRead},
		{"HEAD", true, identity.PermissionRead},
		{"POST", false, identity.PermissionWrite},
		{"DELETE", false, identity.PermissionWrite},
		{"POST", true, identity.PermissionDeploy},
	}
	for _, c := range cases {
		if got := required(c.method, c.deploy); got != c.want {
			t.Errorf("%s deploy=%v: %s, want %s", c.method, c.deploy, got, c.want)
		}
	}
}

func TestEffectivePermissions(t *testing.T) {
	session := identity.Principal{MemberID: 1}
	token := func(ps ...identity.Permission) identity.Principal { return identity.TokenPrincipal(1, 2, ps...) }
	cases := []struct {
		name      string
		principal identity.Principal
		member    domain.Permissions
		want      domain.Permissions
	}{
		{"a Session keeps the viewer's", session, viewer, viewer},
		{"a Session keeps administrator as it is", session, admin, admin},
		{"a viewer's root token", token("root"), viewer, viewer},
		{"an admin's root token", token("root"), admin, domain.AllPermissions},
		{"a member's read token", token("read"), member, viewer},
		{"a member's read:sensitive token", token("read", "read:sensitive"), member,
			domain.Of(domain.PermissionViewResources, domain.PermissionSeeSecrets)},
		{"a member's write token", token("write"), member,
			domain.Of(domain.PermissionViewResources, domain.PermissionManageApplications)},
		{"a member's deploy token", token("deploy"), member,
			domain.Of(domain.PermissionViewResources, domain.PermissionDeploy)},
		{"a viewer's write token", token("write", "deploy", "read:sensitive"), viewer, viewer},
		{"an admin's write token loses administrator", token("write", "deploy", "read:sensitive"), admin,
			domain.AllPermissions.Without(domain.Of(domain.PermissionAdministrator))},
		{"an admin's read token", token("read"), admin, viewer},
	}
	for _, c := range cases {
		if got := effective(c.principal, c.member); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got.Keys(), c.want.Keys())
		}
	}
}

func TestTokenPermissionOfAPermission(t *testing.T) {
	cases := map[domain.Permission]identity.Permission{
		domain.PermissionAdministrator:      identity.PermissionRoot,
		domain.PermissionViewResources:      identity.PermissionRead,
		domain.PermissionSeeSecrets:         identity.PermissionReadSensitive,
		domain.PermissionDeploy:             identity.PermissionDeploy,
		domain.PermissionManageApplications: identity.PermissionWrite,
		domain.PermissionManageServers:      identity.PermissionWrite,
	}
	for p, want := range cases {
		if got := tokenPermission(p); got != want {
			t.Errorf("%s: %s, want %s", p.Key(), got, want)
		}
	}
}

func TestGuildAsked(t *testing.T) {
	session := identity.Principal{MemberID: 1}
	token := identity.TokenPrincipal(1, 2, identity.PermissionRead)
	desktop := identity.Principal{MemberID: 1, DesktopID: 3}
	cases := []struct {
		name          string
		principal     identity.Principal
		header        string
		cookie        uint64
		named, wanted uint64
		ok            bool
	}{
		{"a Session would like its cookie's Guild", session, "7", 5, 0, 5, true},
		{"an API token names its own Guild", token, "7", 5, 2, 5, true},
		{"a Desktop key names the header's Guild", desktop, "7", 5, 7, 0, true},
		{"a Desktop key without the header takes the first", desktop, "", 5, 0, 0, true},
		{"a Desktop key with a header that is no id", desktop, "abc", 0, 0, 0, false},
		{"a Desktop key with Guild 0", desktop, "0", 0, 0, 0, false},
	}
	for _, c := range cases {
		named, wanted, ok := guildAsked(c.principal, c.header, c.cookie)
		if named != c.named || wanted != c.wanted || ok != c.ok {
			t.Errorf("%s: %d %d %v, want %d %d %v", c.name, named, wanted, ok, c.named, c.wanted, c.ok)
		}
	}
}

func TestAuthRefuses(t *testing.T) {
	session := identity.Principal{MemberID: 1}
	token := identity.TokenPrincipal(1, 2, identity.PermissionRoot)
	desktop := identity.Principal{MemberID: 1, DesktopID: 3}
	agent := identity.Principal{AgentID: 4, RunID: 9, GuildID: 1}
	cases := []struct {
		name      string
		auth      Auth
		principal identity.Principal
		refused   bool
	}{
		{"a Session on a SelfService route", Auth{SelfService: true}, session, false},
		{"an API token on a SelfService route", Auth{SelfService: true}, token, true},
		{"a Desktop key on a SelfService route", Auth{SelfService: true}, desktop, true},
		{"a Desktop key where Desktop lets it", Auth{SelfService: true, Desktop: true}, desktop, false},
		{"an API token where Desktop lets a Desktop key", Auth{SelfService: true, Desktop: true}, token, true},
		{"a Desktop key on a Guild's route", Auth{}, desktop, false},
		{"a Desktop key on GET /api/me", Auth{Guildless: true}, desktop, false},
		{"a Run key on a Guild's route", Auth{}, agent, true},
		{"a Run key where Agents lets it", Auth{Agents: true}, agent, false},
		{"a Session where Agents lets a Run key", Auth{Agents: true}, session, false},
		{"a Run key on a SelfService route that lets Agents", Auth{SelfService: true, Agents: true}, agent, true},
		{"a Run key on GET /api/me", Auth{Guildless: true}, agent, true},
		{"a Run key where Desktop lets a Desktop key", Auth{SelfService: true, Desktop: true}, agent, true},
	}
	for _, c := range cases {
		if got := c.auth.refused(c.principal) != ""; got != c.refused {
			t.Errorf("%s: refused %v, want %v", c.name, got, c.refused)
		}
	}
}

func TestDesktopKeyIsUncapped(t *testing.T) {
	desktop := identity.Principal{MemberID: 1, DesktopID: 3}
	if got := effective(desktop, admin); got != admin {
		t.Errorf("a Desktop key keeps the Member's Permissions: %v", got.Keys())
	}
	if !desktop.Allows(identity.PermissionWrite) {
		t.Error("a Desktop key is not limited by Token permissions")
	}
}

type fakeGuilds struct{ app.Guilds }

func (fakeGuilds) ByID(_ context.Context, id uint64) (domain.Guild, bool, error) {
	return domain.Guild{ID: id, Name: "Bakers"}, id == 1, nil
}

// fakeAgents holds the Agent memberships of Guild 1, by Agent id.
type fakeAgents struct {
	app.Memberships
	held map[uint64][]uint64
}

func (f fakeAgents) OfAgent(_ context.Context, guildID, agentID uint64) (domain.Membership, bool, error) {
	roles, ok := f.held[agentID]
	return domain.Membership{GuildID: guildID, AgentID: agentID, RoleIDs: roles}, ok && guildID == 1, nil
}

type fakeRoles struct {
	app.Roles
	all []domain.Role
}

func (f fakeRoles) ForGuild(context.Context, uint64) ([]domain.Role, error) { return f.all, nil }

// TestAgentPlace: an Agent principal acts in its Run's Guild with what its
// Roles allow (the Base role's included), never in another Guild, reads
// only with view_resources, and a Project override on a Role it holds
// counts for it as for a person.
func TestAgentPlace(t *testing.T) {
	const base, reader, writer = 1, 2, 3
	roles := fakeRoles{all: []domain.Role{
		{ID: base, Base: true},
		{ID: reader, Permissions: viewer},
		{ID: writer, Permissions: domain.Of(domain.PermissionManageWork)},
	}}
	// Agent 4 reads and writes; Agent 5 only writes; Agent 6 is not in the Guild.
	agents := fakeAgents{held: map[uint64][]uint64{4: {reader, writer}, 5: {writer}}}
	overrides := fakeOverrides{{GuildID: 1, ProjectID: 2, RoleID: reader, Deny: viewer}}
	s := app.NewService(fakeGuilds{}, agents, roles, nil, nil, overrides, nil)
	run := func(agentID uint64) identity.Principal {
		return identity.Principal{AgentID: agentID, RunID: 9, GuildID: 1}
	}
	cases := []struct {
		name      string
		principal identity.Principal
		header    string
		method    string
		refused   string
	}{
		{"reading with view_resources", run(4), "", "GET", ""},
		{"naming its own Guild", run(4), "1", "GET", ""},
		{"naming another Guild", run(4), "2", "GET", "not a member of this guild"},
		{"reading without view_resources", run(5), "", "GET", "you need the View resources permission"},
		{"changing without view_resources", run(5), "", "POST", ""},
		{"an Agent no longer in the Guild", run(6), "", "GET", "not a member of this guild"},
		{"a Run of a deleted Guild", identity.Principal{AgentID: 4, RunID: 9, GuildID: 7}, "", "GET", "not a member of this guild"},
	}
	for _, c := range cases {
		pl, msg, err := agentPlace(context.Background(), s, c.principal, c.header, c.method)
		if err != nil {
			t.Fatal(err)
		}
		if msg != c.refused {
			t.Errorf("%s: refused %q, want %q", c.name, msg, c.refused)
			continue
		}
		if msg == "" && (pl.guild.ID != 1 || pl.principal.AgentID != c.principal.AgentID) {
			t.Errorf("%s: acts in Guild %d as Agent %d", c.name, pl.guild.ID, pl.principal.AgentID)
		}
	}

	pl, _, _ := agentPlace(context.Background(), s, run(4), "", "GET")
	if want := viewer.Union(domain.Of(domain.PermissionManageWork)); pl.permissions != want {
		t.Errorf("Agent 4 may %v, want %v", pl.permissions.Keys(), want.Keys())
	}
	if pl.principal.MemberID != 0 {
		t.Error("an Agent principal names a Member")
	}
	// Projects 1 and 2: the override denies the reader Role view_resources
	// in Project 2, so the Agent finds it no more than another Guild's.
	m := InProject{Service: s, Name: "project", ProjectOf: func(_ context.Context, id uint64) (uint64, uint64, bool, error) {
		return id, 1, id <= 2, nil
	}}
	for id, want := range map[uint64]bool{1: true, 2: false, 3: false} {
		if _, found, err := m.resolve(context.Background(), pl, id); err != nil || found != want {
			t.Errorf("project %d: found %v (%v), want %v", id, found, err, want)
		}
	}
}
