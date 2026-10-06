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
	if tok, err := NewAPIToken(1, 1, admin, "  ci deploy  ", nil, nil, now); err != nil || tok.Name != "ci deploy" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	for _, name := range []string{"", "   ", "ci", strings.Repeat("x", 256)} {
		if _, err := NewAPIToken(1, 1, admin, name, nil, nil, now); !errors.Is(err, ErrInvalidTokenName) {
			t.Errorf("%q: %v", name, err)
		}
	}
	past := now.Add(-time.Second)
	if _, err := NewAPIToken(1, 1, admin, "old", nil, &past, now); !errors.Is(err, ErrTokenExpiryPassed) {
		t.Errorf("expiry in the past: %v", err)
	}
	future := now.Add(time.Hour)
	if tok, err := NewAPIToken(1, 1, admin, "new", nil, &future, now); err != nil || !tok.ExpiresAt.Equal(future) {
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

func TestNewAPITokenPermissionCaps(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		perms MemberPermissions
		p     Permission
		ok    bool
	}{
		{admin, PermissionRoot, true},
		{member, PermissionRoot, false},
		{member, PermissionWrite, true},
		{member, PermissionDeploy, true},
		{member, PermissionReadSensitive, true},
		{viewer, PermissionWrite, false},
		{viewer, PermissionDeploy, false},
		{viewer, PermissionReadSensitive, false},
		{viewer, PermissionRead, true},
	} {
		_, err := NewAPIToken(1, 1, c.perms, "token", []Permission{c.p}, nil, now)
		if c.ok != (err == nil) {
			t.Errorf("%v granting %s: %v", c.perms, c.p, err)
		}
		if !c.ok && (!errors.Is(err, ErrCannotGrant) || err.Error() != "your permissions cannot grant "+string(c.p)) {
			t.Errorf("%v granting %s: wrong error %v", c.perms, c.p, err)
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

var (
	admin  = MemberPermissions{"administrator"}
	member = MemberPermissions{"view_resources", "see_secrets", "deploy", "manage_applications"}
	viewer = MemberPermissions{"view_resources"}
)
