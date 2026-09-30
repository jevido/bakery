package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestCounterparts(t *testing.T) {
	for _, c := range []struct {
		domains []string
		mode    WwwRedirect
		want    map[string]string
	}{
		{[]string{"example.com"}, WwwOff, map[string]string{}},
		{[]string{"example.com", "app.example.org"}, ToApex, map[string]string{"www.example.com": "example.com", "www.app.example.org": "app.example.org"}},
		{[]string{"www.example.com", "example.org"}, ToApex, map[string]string{"www.example.org": "example.org"}},
		{[]string{"www.example.com", "example.org"}, ToWww, map[string]string{"example.com": "www.example.com"}},
		// Both listed: both are served, nothing redirects.
		{[]string{"example.com", "www.example.com"}, ToWww, map[string]string{}},
		{[]string{"www.com"}, ToWww, map[string]string{}},
	} {
		if got := Counterparts(c.domains, c.mode); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Counterparts(%v, %s) = %v, want %v", c.domains, c.mode, got, c.want)
		}
	}
}

func TestRouteSettingsCheck(t *testing.T) {
	s, err := RouteSettings{ApplicationID: 1}.Check()
	if err != nil || s.WwwRedirect != WwwOff {
		t.Fatalf("empty is off: %v %v", s, err)
	}
	var fe *FieldError
	if _, err := (RouteSettings{WwwRedirect: "sideways"}).Check(); !errors.As(err, &fe) || fe.Field != "www_redirect" {
		t.Fatalf("unknown mode: %v", err)
	}
}
