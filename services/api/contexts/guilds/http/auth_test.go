package http

import (
	"testing"

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
