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
