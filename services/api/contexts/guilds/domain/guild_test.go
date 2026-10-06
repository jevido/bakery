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
		g, err := NewGuild(c.name, c.description)
		if !errors.Is(err, c.want) {
			t.Errorf("NewGuild(%q, %q): err = %v, want %v", c.name, c.description, err, c.want)
		}
		if err == nil && (g.Name != strings.TrimSpace(c.name) || g.Description != strings.TrimSpace(c.description)) {
			t.Errorf("NewGuild(%q, %q) = %+v: not trimmed", c.name, c.description, g)
		}
	}
}

func TestRenameKeepsTheOldNameOnError(t *testing.T) {
	g, _ := NewGuild("Bakers", "")
	if err := g.Rename(""); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("err = %v", err)
	}
	if g.Name != "Bakers" {
		t.Errorf("name = %q", g.Name)
	}
}

func TestRoleRights(t *testing.T) {
	cases := []struct {
		role                    Role
		write, secrets, isAdmin bool
	}{
		{RoleViewer, false, false, false},
		{RoleMember, true, true, false},
		{RoleAdmin, true, true, true},
	}
	for _, c := range cases {
		if c.role.CanWrite() != c.write || c.role.CanSeeSecrets() != c.secrets || c.role.IsAdmin() != c.isAdmin {
			t.Errorf("%s: write=%v secrets=%v admin=%v", c.role, c.role.CanWrite(), c.role.CanSeeSecrets(), c.role.IsAdmin())
		}
	}
	for _, s := range []string{"owner", "Admin", ""} {
		if _, err := ParseRole(s); !errors.Is(err, ErrInvalidRole) {
			t.Errorf("ParseRole(%q): %v", s, err)
		}
	}
}

func TestKeepsAnAdmin(t *testing.T) {
	ms := []Membership{{MemberID: 1, Role: RoleAdmin}, {MemberID: 2, Role: RoleMember}}
	cases := []struct {
		member uint64
		to     Role
		want   error
	}{
		{1, RoleMember, ErrLastAdmin},
		{1, "", ErrLastAdmin},
		{2, RoleViewer, nil},
		{2, "", nil},
		{2, RoleAdmin, nil},
	}
	for _, c := range cases {
		if err := KeepsAnAdmin(ms, c.member, c.to); !errors.Is(err, c.want) {
			t.Errorf("member %d to %q: err = %v, want %v", c.member, c.to, err, c.want)
		}
	}
	two := append(ms, Membership{MemberID: 3, Role: RoleAdmin})
	if err := KeepsAnAdmin(two, 1, ""); err != nil {
		t.Errorf("a second admin stays: %v", err)
	}
}

func TestCanManage(t *testing.T) {
	dev := Membership{MemberID: 3, Role: RoleMember}
	cases := []struct {
		actor         uint64
		role          Role
		target        Membership
		instanceAdmin bool
		want          error
	}{
		{1, RoleAdmin, dev, false, nil},
		{1, RoleMember, dev, false, ErrNotAdmin},
		{1, RoleAdmin, Membership{MemberID: 2, Role: RoleAdmin}, true, ErrInstanceAdminFixed},
		{3, RoleAdmin, dev, false, ErrSelf},
	}
	for _, c := range cases {
		if err := CanManage(c.actor, c.role, c.target, c.instanceAdmin); !errors.Is(err, c.want) {
			t.Errorf("%d (%s) manages %d: %v, want %v", c.actor, c.role, c.target.MemberID, err, c.want)
		}
	}
}
