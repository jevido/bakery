package http

import (
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

func TestRefusal(t *testing.T) {
	cases := []struct {
		method string
		role   domain.Role
		refuse bool
	}{
		{"GET", domain.RoleViewer, false},
		{"HEAD", domain.RoleViewer, false},
		{"POST", domain.RoleViewer, true},
		{"PATCH", domain.RoleViewer, true},
		{"DELETE", domain.RoleViewer, true},
		{"POST", domain.RoleMember, false},
		{"DELETE", domain.RoleAdmin, false},
	}
	for _, c := range cases {
		if got := refusal(c.method, c.role) != ""; got != c.refuse {
			t.Errorf("%s as %s: refused=%v, want %v", c.method, c.role, got, c.refuse)
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
	if refusal("POST", domain.RoleViewer) == "" {
		t.Error("a viewer's token with deploy still cannot change anything")
	}
	viewer := place{principal: identity.TokenPrincipal(1, 2, "root"), role: domain.RoleViewer}
	if viewer.role.CanSeeSecrets() && viewer.principal.Allows(identity.PermissionReadSensitive) {
		t.Error("a viewer's root token sees Secrets")
	}
}
