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
