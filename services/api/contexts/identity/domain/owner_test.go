package domain

import (
	"errors"
	"testing"
)

func TestNewOwner(t *testing.T) {
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
		o, err := NewOwner(c.name, c.email, c.password)
		if !errors.Is(err, c.want) {
			t.Errorf("NewOwner(%q, %q): err = %v, want %v", c.name, c.email, err, c.want)
		}
		if err == nil && o.Email != "ada@example.com" {
			t.Errorf("email not normalised: %q", o.Email)
		}
	}
}
