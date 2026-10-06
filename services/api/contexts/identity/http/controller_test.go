package http

import (
	"testing"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
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
		{"PUT", domain.RoleOwner, false},
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
		want   domain.Permission
	}{
		{"GET", false, domain.PermissionRead},
		{"HEAD", true, domain.PermissionRead},
		{"POST", false, domain.PermissionWrite},
		{"DELETE", false, domain.PermissionWrite},
		{"POST", true, domain.PermissionDeploy},
	}
	for _, c := range cases {
		if got := (Auth{Deploy: c.deploy}).required(c.method); got != c.want {
			t.Errorf("%s deploy=%v: %s, want %s", c.method, c.deploy, got, c.want)
		}
	}
}

func TestPrincipalAllows(t *testing.T) {
	session := principal{role: domain.RoleViewer}
	if !session.allows(domain.PermissionReadSensitive) {
		t.Error("a Session is not limited by Permissions")
	}
	deploy := principal{role: domain.RoleOwner, token: &domain.APIToken{Permissions: []domain.Permission{domain.PermissionDeploy}}}
	if deploy.allows(domain.PermissionRead) || deploy.allows(domain.PermissionWrite) || !deploy.allows(domain.PermissionDeploy) {
		t.Error("a deploy token only deploys")
	}
}
