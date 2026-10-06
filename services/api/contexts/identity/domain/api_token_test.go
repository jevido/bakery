package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewAPIToken(t *testing.T) {
	now := time.Now()
	if tok, err := NewAPIToken(1, 1, RoleAdmin, "  ci deploy  ", nil, nil, now); err != nil || tok.Name != "ci deploy" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	for _, name := range []string{"", "   ", "ci", strings.Repeat("x", 256)} {
		if _, err := NewAPIToken(1, 1, RoleAdmin, name, nil, nil, now); !errors.Is(err, ErrInvalidTokenName) {
			t.Errorf("%q: %v", name, err)
		}
	}
	past := now.Add(-time.Second)
	if _, err := NewAPIToken(1, 1, RoleAdmin, "old", nil, &past, now); !errors.Is(err, ErrTokenExpiryPassed) {
		t.Errorf("expiry in the past: %v", err)
	}
	future := now.Add(time.Hour)
	if tok, err := NewAPIToken(1, 1, RoleAdmin, "new", nil, &future, now); err != nil || !tok.ExpiresAt.Equal(future) {
		t.Errorf("expiry in the future: %+v %v", tok, err)
	}
}

func TestNormalisePermissions(t *testing.T) {
	for _, c := range []struct {
		in, want []Permission
	}{
		{nil, []Permission{PermissionRead}},
		{[]Permission{PermissionWrite, PermissionRoot, PermissionRead}, []Permission{PermissionRoot}},
		{[]Permission{PermissionReadSensitive}, []Permission{PermissionRead, PermissionReadSensitive}},
		{[]Permission{PermissionWrite, PermissionDeploy, PermissionWrite}, []Permission{PermissionDeploy, PermissionWrite}},
		{[]Permission{PermissionDeploy}, []Permission{PermissionDeploy}},
	} {
		got, err := NormalisePermissions(c.in)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%v: got %v, %v; want %v", c.in, got, err, c.want)
		}
	}
	if _, err := NormalisePermissions([]Permission{"write:sensitive"}); !errors.Is(err, ErrUnknownPermission) {
		t.Errorf("unknown: %v", err)
	}
}

func TestNewAPITokenRoleCaps(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		role Role
		p    Permission
		ok   bool
	}{
		{RoleAdmin, PermissionRoot, true},
		{RoleMember, PermissionRoot, false},
		{RoleMember, PermissionWrite, true},
		{RoleMember, PermissionDeploy, true},
		{RoleMember, PermissionReadSensitive, true},
		{RoleViewer, PermissionWrite, false},
		{RoleViewer, PermissionDeploy, false},
		{RoleViewer, PermissionReadSensitive, false},
		{RoleViewer, PermissionRead, true},
	} {
		_, err := NewAPIToken(1, 1, c.role, "token", []Permission{c.p}, nil, now)
		if c.ok != (err == nil) {
			t.Errorf("%s granting %s: %v", c.role, c.p, err)
		}
		if !c.ok && (!errors.Is(err, ErrRoleCannotGrant) || err.Error() != "your role cannot grant "+string(c.p)) {
			t.Errorf("%s granting %s: wrong error %v", c.role, c.p, err)
		}
	}
}

func TestAPITokenAllowsAndExpires(t *testing.T) {
	root := APIToken{Permissions: []Permission{PermissionRoot}}
	read := APIToken{Permissions: []Permission{PermissionRead}}
	if !root.Allows(PermissionDeploy) || !root.Allows(PermissionReadSensitive) {
		t.Error("root allows everything")
	}
	if read.Allows(PermissionReadSensitive) || read.Allows(PermissionWrite) || !read.Allows(PermissionRead) {
		t.Error("read allows only reading")
	}
	if !read.ReadOnly() || root.ReadOnly() {
		t.Error("read-only")
	}
	now := time.Now()
	if root.Expired(now) {
		t.Error("never expires")
	}
	root.ExpiresAt = &now
	if !root.Expired(now) || root.Expired(now.Add(-time.Second)) {
		t.Error("expires at ExpiresAt")
	}
}
