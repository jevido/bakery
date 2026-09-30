package domain

import (
	"errors"
	"testing"
)

func TestNewMember(t *testing.T) {
	cases := []struct {
		name, email, password string
		role                  Role
		want                  error
	}{
		{"Ada", " Ada@Example.com ", "correct horse", RoleOwner, nil},
		{"  ", "ada@example.com", "correct horse", RoleOwner, ErrInvalidName},
		{"Ada", "not-an-email", "correct horse", RoleOwner, ErrInvalidEmail},
		{"Ada", "Ada <ada@example.com>", "correct horse", RoleOwner, ErrInvalidEmail},
		{"Ada", "ada@example.com", "short", RoleOwner, ErrPasswordTooShort},
		{"Ada", "ada@example.com", "correct horse", Role("root"), ErrInvalidRole},
	}
	for _, c := range cases {
		m, err := NewMember(c.name, c.email, c.password, c.role)
		if !errors.Is(err, c.want) {
			t.Errorf("NewMember(%q, %q, %q): err = %v, want %v", c.name, c.email, c.role, err, c.want)
		}
		if err == nil && m.Email != "ada@example.com" {
			t.Errorf("email not normalised: %q", m.Email)
		}
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
		{RoleOwner, true, true, true},
	}
	for _, c := range cases {
		if c.role.CanWrite() != c.write || c.role.CanSeeSecrets() != c.secrets || c.role.IsAdmin() != c.isAdmin {
			t.Errorf("%s: write=%v secrets=%v admin=%v", c.role, c.role.CanWrite(), c.role.CanSeeSecrets(), c.role.IsAdmin())
		}
	}
	if _, err := ParseRole("admin"); err != nil {
		t.Error(err)
	}
	if _, err := ParseRole("Admin"); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("ParseRole(Admin): %v", err)
	}
}
