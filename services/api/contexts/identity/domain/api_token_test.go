package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewAPIToken(t *testing.T) {
	now := time.Now()
	if tok, err := NewAPIToken(1, "  ci  ", false, now); err != nil || tok.Name != "ci" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	for _, name := range []string{"", "   ", strings.Repeat("x", 65)} {
		if _, err := NewAPIToken(1, name, false, now); !errors.Is(err, ErrInvalidTokenName) {
			t.Errorf("%q: %v", name, err)
		}
	}
}

func TestEffectiveRole(t *testing.T) {
	if r := (APIToken{ReadOnly: true}).EffectiveRole(RoleOwner); r != RoleViewer {
		t.Errorf("read-only: %s", r)
	}
	if r := (APIToken{}).EffectiveRole(RoleMember); r != RoleMember {
		t.Errorf("full: %s", r)
	}
}
