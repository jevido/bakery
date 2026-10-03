package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// FieldError is a broken rule on one input field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Redirect says which of a Domain and its www counterpart redirects to the
// other, in Coolify's values.
type Redirect string

const (
	// Both serves only the Domains themselves; neither redirects.
	Both Redirect = "both"
	// NonWww redirects www.<Domain> to each Domain without www.
	NonWww Redirect = "non-www"
	// Www redirects <Domain> to each Domain that starts with www.
	Www Redirect = "www"
)

// RouteSettings is how the Proxy treats one Application's traffic. It is
// stored apart from the Route, since it exists before the first Deployment.
type RouteSettings struct {
	ApplicationID   uint64
	Redirect        Redirect
	ResponseHeaders []ResponseHeader
	BasicAuth       BasicAuth
}

// ResponseHeader is set (not added) on every response.
type ResponseHeader struct {
	Name  string
	Value string
}

// BasicAuth asks for one username and password before any request is
// proxied. Only the bcrypt hash of the password is kept. Switched off, the
// username and hash stay, so switching it on again needs no new password.
type BasicAuth struct {
	Enabled      bool
	Username     string
	PasswordHash string
}

// MaxResponseHeaders is how many Response headers one Route may set.
const MaxResponseHeaders = 20

// headerToken is an HTTP field name (RFC 9110 token).
var headerToken = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

// forbiddenHeaders belong to the connection or the body, not the app.
var forbiddenHeaders = map[string]bool{
	"connection": true, "keep-alive": true, "te": true, "trailer": true,
	"transfer-encoding": true, "upgrade": true, "content-length": true,
}

// DefaultRouteSettings is everything off.
func DefaultRouteSettings(applicationID uint64) RouteSettings {
	return RouteSettings{ApplicationID: applicationID, Redirect: Both}
}

// Check validates the settings; an empty Redirect means both.
func (s RouteSettings) Check() (RouteSettings, error) {
	switch s.Redirect {
	case "":
		s.Redirect = Both
	case Both, NonWww, Www:
	default:
		return s, invalid("redirect", "redirect must be both, www or non-www")
	}
	if len(s.ResponseHeaders) > MaxResponseHeaders {
		return s, invalid("response_headers", "at most %d response headers", MaxResponseHeaders)
	}
	seen := map[string]bool{}
	headers := make([]ResponseHeader, len(s.ResponseHeaders))
	for i, h := range s.ResponseHeaders {
		h.Name = strings.TrimSpace(h.Name)
		lower := strings.ToLower(h.Name)
		switch {
		case !headerToken.MatchString(h.Name):
			return s, invalid("response_headers", "%q is not a valid header name", h.Name)
		case forbiddenHeaders[lower] || strings.HasPrefix(lower, "proxy-"):
			return s, invalid("response_headers", "%s cannot be set by the proxy", h.Name)
		case seen[lower]:
			return s, invalid("response_headers", "%s is set twice", h.Name)
		case strings.ContainsAny(h.Value, "\r\n\x00"):
			return s, invalid("response_headers", "the value of %s must be on one line", h.Name)
		case len(h.Value) > 1024:
			return s, invalid("response_headers", "the value of %s is at most 1024 characters", h.Name)
		}
		seen[lower] = true
		headers[i] = h
	}
	s.ResponseHeaders = headers
	a := s.BasicAuth
	a.Username = strings.TrimSpace(a.Username)
	switch {
	case len(a.Username) > 100 || strings.ContainsAny(a.Username, ":\r\n"):
		return s, invalid("basic_auth.username", "username is at most 100 characters, without a colon")
	case a.Enabled && a.Username == "":
		return s, invalid("basic_auth.username", "username is required for basic auth")
	case a.Enabled && a.PasswordHash == "":
		return s, invalid("basic_auth.password", "password is required for basic auth")
	}
	s.BasicAuth = a
	return s, nil
}

var hostnameLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// isHostname is a lowercase DNS name with at least two labels.
func isHostname(s string) bool {
	labels := strings.Split(s, ".")
	if len(s) > 253 || len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if !hostnameLabel.MatchString(l) {
			return false
		}
	}
	return true
}

// Counterparts maps each counterpart host the Redirect adds to the
// Domain it redirects to. A counterpart that is one of the Domains itself,
// or not a hostname of at least two labels (www.com → com), is left out.
func Counterparts(domains []string, mode Redirect) map[string]string {
	out := map[string]string{}
	own := make(map[string]bool, len(domains))
	for _, d := range domains {
		own[d] = true
	}
	for _, d := range domains {
		var counterpart string
		switch {
		case mode == NonWww && !strings.HasPrefix(d, "www."):
			counterpart = "www." + d
		case mode == Www && strings.HasPrefix(d, "www."):
			counterpart = strings.TrimPrefix(d, "www.")
		default:
			continue
		}
		if !own[counterpart] && isHostname(counterpart) {
			out[counterpart] = d
		}
	}
	return out
}
