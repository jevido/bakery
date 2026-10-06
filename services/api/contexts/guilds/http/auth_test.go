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

func TestRefusal(t *testing.T) {
	cases := []struct {
		method string
		perms  domain.Permissions
		refuse bool
	}{
		{"GET", viewer, false},
		{"HEAD", viewer, false},
		{"POST", viewer, true},
		{"PATCH", viewer, true},
		{"DELETE", viewer, true},
		{"POST", member, false},
		{"DELETE", admin, false},
	}
	for _, c := range cases {
		if got := refusal(c.method, c.perms) != ""; got != c.refuse {
			t.Errorf("%s as %s: refused=%v, want %v", c.method, wireRole(c.perms), got, c.refuse)
		}
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

func TestATokensPermissionsAndRoleBothCount(t *testing.T) {
	deploy := identity.TokenPrincipal(1, 2, identity.PermissionDeploy)
	if deploy.Allows(identity.PermissionRead) || !deploy.Allows(identity.PermissionDeploy) {
		t.Error("a deploy token only deploys")
	}
	if refusal("POST", viewer) == "" {
		t.Error("a viewer's token with deploy still cannot change anything")
	}
	p := place{principal: identity.TokenPrincipal(1, 2, "root"), permissions: viewer}
	if p.permissions.Has(domain.PermissionSeeSecrets) && p.principal.Allows(identity.PermissionReadSensitive) {
		t.Error("a viewer's root token sees Secrets")
	}
}
