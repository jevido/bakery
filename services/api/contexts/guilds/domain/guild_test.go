package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewGuild(t *testing.T) {
	cases := []struct {
		name, description string
		want              error
	}{
		{" Bakers ", " bread ", nil},
		{"Bakers", "", nil},
		{"  ", "", ErrInvalidName},
		{strings.Repeat("é", 255), "", nil},
		{strings.Repeat("é", 256), "", ErrInvalidName},
		{"Bakers", strings.Repeat("x", 255), nil},
		{"Bakers", strings.Repeat("x", 256), ErrDescriptionTooLong},
	}
	for _, c := range cases {
		g, err := NewGuild(c.name, c.description, 1)
		if !errors.Is(err, c.want) {
			t.Errorf("NewGuild(%q, %q): err = %v, want %v", c.name, c.description, err, c.want)
		}
		if err == nil && (g.Name != strings.TrimSpace(c.name) || g.Description != strings.TrimSpace(c.description)) {
			t.Errorf("NewGuild(%q, %q) = %+v: not trimmed", c.name, c.description, g)
		}
	}
}

func TestRenameKeepsTheOldNameOnError(t *testing.T) {
	g, _ := NewGuild("Bakers", "", 1)
	if err := g.Rename(""); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("err = %v", err)
	}
	if g.Name != "Bakers" {
		t.Errorf("name = %q", g.Name)
	}
}

func TestPermissions(t *testing.T) {
	member := Of(PermissionViewResources, PermissionSeeSecrets, PermissionDeploy, PermissionManageApplications)
	if !member.Has(PermissionDeploy) || member.Has(PermissionManageServers) {
		t.Errorf("member: %v", member.Keys())
	}
	admin := Of(PermissionAdministrator)
	for p := PermissionAdministrator; p < endOfPermissions; p <<= 1 {
		if !admin.Has(p) {
			t.Errorf("administrator lacks %s", p.Key())
		}
	}
	if Permissions(0).Has(PermissionViewResources) {
		t.Error("no Permissions has view_resources")
	}
	if u := Of(PermissionViewResources).Union(Of(PermissionDeploy)); u != Of(PermissionViewResources, PermissionDeploy) {
		t.Errorf("union = %v", u.Keys())
	}
	keys := AllPermissions.Keys()
	if len(keys) != 14 || keys[0] != "administrator" || keys[13] != "manage_work" {
		t.Errorf("keys = %v", keys)
	}
	back, err := ParsePermissions(keys)
	if err != nil || back != AllPermissions {
		t.Errorf("round trip = %v, %v", back.Keys(), err)
	}
	if got := Permissions(0).Keys(); got == nil || len(got) != 0 {
		t.Errorf("no keys = %#v", got)
	}
	if _, err := ParsePermissions([]string{"deploy", "fly"}); !errors.Is(err, ErrUnknownPermission) {
		t.Errorf("unknown key: %v", err)
	}
	if PermissionDeploy.Key() != "deploy" || Permission(3).Key() != "" {
		t.Error("Key")
	}
}

func TestRoleRenameAndRecolor(t *testing.T) {
	var r Role
	if err := r.Rename("  Deployer "); err != nil || r.Name != "Deployer" {
		t.Errorf("rename: %q, %v", r.Name, err)
	}
	for _, n := range []string{" ", strings.Repeat("é", 101)} {
		if err := r.Rename(n); !errors.Is(err, ErrInvalidRoleName) {
			t.Errorf("rename %q: %v", n, err)
		}
	}
	if err := r.Recolor("#E74C3C"); err != nil || r.Color != "#e74c3c" {
		t.Errorf("recolor: %q, %v", r.Color, err)
	}
	for _, c := range []string{"red", "#fff", "#gggggg", ""} {
		if err := r.Recolor(c); !errors.Is(err, ErrInvalidRoleColor) {
			t.Errorf("recolor %q: %v", c, err)
		}
	}
}

// seeded are a Guild's seeded Roles with ids 1 to 4.
func seeded() []Role {
	roles := SeedRoles(7)
	for i := range roles {
		roles[i].ID = uint64(i + 1)
	}
	return roles
}

func TestSeededRoles(t *testing.T) {
	roles := seeded()
	for _, c := range []struct {
		former string
		id     uint64
	}{{"viewer", 2}, {"member", 3}, {"admin", 4}} {
		r, ok, err := SeededRole(roles, c.former)
		if err != nil || !ok || r.ID != c.id {
			t.Errorf("%s: %+v %v %v", c.former, r, ok, err)
		}
	}
	for _, s := range []string{"owner", "Admin", ""} {
		if _, _, err := SeededRole(roles, s); !errors.Is(err, ErrInvalidRole) {
			t.Errorf("SeededRole(%q): %v", s, err)
		}
	}
	if _, ok, _ := SeededRole(roles[:2], "admin"); ok {
		t.Error("found a deleted Admin")
	}
}

func TestPermissionsOf(t *testing.T) {
	roles := seeded()
	cases := []struct {
		held                          []uint64
		write, secrets, isAdmin, view bool
	}{
		{nil, false, false, false, false},
		{[]uint64{2}, false, false, false, true},
		{[]uint64{3}, true, true, false, true},
		{[]uint64{4}, true, true, true, true},
		{[]uint64{2, 3}, true, true, false, true},
	}
	for _, c := range cases {
		p := PermissionsOf(roles, Membership{RoleIDs: c.held})
		if p.Has(PermissionManageApplications) != c.write || p.Has(PermissionSeeSecrets) != c.secrets ||
			p.Has(PermissionAdministrator) != c.isAdmin || p.Has(PermissionViewResources) != c.view {
			t.Errorf("%v: %v", c.held, p.Keys())
		}
	}
	roles[0].Permissions = Of(PermissionViewResources)
	if !PermissionsOf(roles, Membership{}).Has(PermissionViewResources) {
		t.Error("the Base role counts for everyone")
	}
}
