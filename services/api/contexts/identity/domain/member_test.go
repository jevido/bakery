package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewMember(t *testing.T) {
	cases := []struct {
		name, email, password string
		want                  error
	}{
		{"Ada", " Ada@Example.com ", "correct horse", nil},
		{"  ", "ada@example.com", "correct horse", ErrInvalidName},
		{"Ada", "not-an-email", "correct horse", ErrInvalidEmail},
		{"Ada", "Ada <ada@example.com>", "correct horse", ErrInvalidEmail},
		{"Ada", "ada@example.com", "short", ErrPasswordTooShort},
	}
	for _, c := range cases {
		m, err := NewMember(c.name, c.email, c.password)
		if !errors.Is(err, c.want) {
			t.Errorf("NewMember(%q, %q): err = %v, want %v", c.name, c.email, err, c.want)
		}
		if err == nil && m.Email != "ada@example.com" {
			t.Errorf("email not normalised: %q", m.Email)
		}
	}
}

func TestRoleRights(t *testing.T) {
	cases := []struct {
		role           Role
		write, isAdmin bool
	}{
		{RoleViewer, false, false},
		{RoleMember, true, false},
		{RoleAdmin, true, true},
	}
	for _, c := range cases {
		if c.role.CanWrite() != c.write || c.role.IsAdmin() != c.isAdmin {
			t.Errorf("%s: write=%v admin=%v", c.role, c.role.CanWrite(), c.role.IsAdmin())
		}
	}
	if _, err := ParseRole("admin"); err != nil {
		t.Error(err)
	}
	for _, s := range []string{"Admin", "owner"} {
		if _, err := ParseRole(s); !errors.Is(err, ErrInvalidRole) {
			t.Errorf("ParseRole(%s): %v", s, err)
		}
	}
}

func TestSessionCounts(t *testing.T) {
	var m Member
	if !m.SessionCounts(time.Unix(1, 0)) {
		t.Fatal("no stamp: every Session counts")
	}
	stampedAt := time.Unix(1000, 700_000_000)
	m.SessionsValidFrom = SessionsValidFromNow(stampedAt)
	// Sessions carry whole seconds.
	if m.SessionCounts(time.Unix(999, 0)) {
		t.Error("a Session of the second before counts")
	}
	if !m.SessionCounts(time.Unix(1000, 0)) {
		t.Error("the fresh Session of the same second does not count")
	}
	if !m.SessionCounts(time.Unix(1001, 0)) {
		t.Error("a later Session does not count")
	}
}
