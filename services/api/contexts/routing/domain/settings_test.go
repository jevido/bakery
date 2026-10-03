package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestCounterparts(t *testing.T) {
	for _, c := range []struct {
		domains []string
		mode    Redirect
		want    map[string]string
	}{
		{[]string{"example.com"}, Both, map[string]string{}},
		{[]string{"example.com", "app.example.org"}, NonWww, map[string]string{"www.example.com": "example.com", "www.app.example.org": "app.example.org"}},
		{[]string{"www.example.com", "example.org"}, NonWww, map[string]string{"www.example.org": "example.org"}},
		{[]string{"www.example.com", "example.org"}, Www, map[string]string{"example.com": "www.example.com"}},
		// Both listed: both are served, nothing redirects.
		{[]string{"example.com", "www.example.com"}, Www, map[string]string{}},
		{[]string{"www.com"}, Www, map[string]string{}},
	} {
		if got := Counterparts(c.domains, c.mode); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Counterparts(%v, %s) = %v, want %v", c.domains, c.mode, got, c.want)
		}
	}
}

func TestRouteSettingsCheck(t *testing.T) {
	s, err := RouteSettings{ApplicationID: 1}.Check()
	if err != nil || s.Redirect != Both {
		t.Fatalf("empty is off: %v %v", s, err)
	}
	var fe *FieldError
	if _, err := (RouteSettings{Redirect: "sideways"}).Check(); !errors.As(err, &fe) || fe.Field != "redirect" {
		t.Fatalf("unknown mode: %v", err)
	}
}

func TestResponseHeadersAndBasicAuthCheck(t *testing.T) {
	ok := RouteSettings{ResponseHeaders: []ResponseHeader{{" X-Frame-Options ", "DENY"}}}
	s, err := ok.Check()
	if err != nil || s.ResponseHeaders[0].Name != "X-Frame-Options" {
		t.Fatalf("valid: %v %+v", err, s)
	}
	many := make([]ResponseHeader, MaxResponseHeaders+1)
	for i := range many {
		many[i] = ResponseHeader{Name: "X-" + string(rune('A'+i)), Value: "1"}
	}
	for i, c := range []struct {
		s     RouteSettings
		field string
	}{
		{RouteSettings{ResponseHeaders: []ResponseHeader{{"Content-Length", "1"}}}, "response_headers"},
		{RouteSettings{ResponseHeaders: []ResponseHeader{{"Proxy-Authorization", "1"}}}, "response_headers"},
		{RouteSettings{ResponseHeaders: []ResponseHeader{{"X A", "1"}}}, "response_headers"},
		{RouteSettings{ResponseHeaders: []ResponseHeader{{"X-A", "1"}, {"x-a", "2"}}}, "response_headers"},
		{RouteSettings{ResponseHeaders: []ResponseHeader{{"X-A", "a\r\nSet-Cookie: x"}}}, "response_headers"},
		{RouteSettings{ResponseHeaders: many}, "response_headers"},
		{RouteSettings{BasicAuth: BasicAuth{Enabled: true, PasswordHash: "h"}}, "basic_auth.username"},
		{RouteSettings{BasicAuth: BasicAuth{Enabled: true, Username: "a:b", PasswordHash: "h"}}, "basic_auth.username"},
		{RouteSettings{BasicAuth: BasicAuth{Enabled: true, Username: "me"}}, "basic_auth.password"},
	} {
		var fe *FieldError
		if _, err := c.s.Check(); !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("case %d: %v, want field %s", i, err, c.field)
		}
	}
}
