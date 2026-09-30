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

// WwwRedirect says which of a Domain and its www counterpart redirects to
// the other.
type WwwRedirect string

const (
	// WwwOff serves only the Domains themselves.
	WwwOff WwwRedirect = "off"
	// ToApex redirects www.<Domain> to each Domain without www.
	ToApex WwwRedirect = "to_apex"
	// ToWww redirects <Domain> to each Domain that starts with www.
	ToWww WwwRedirect = "to_www"
)

// RouteSettings is how the Proxy treats one Application's traffic. It is
// stored apart from the Route, since it exists before the first Deployment.
type RouteSettings struct {
	ApplicationID uint64
	WwwRedirect   WwwRedirect
}

// DefaultRouteSettings is everything off.
func DefaultRouteSettings(applicationID uint64) RouteSettings {
	return RouteSettings{ApplicationID: applicationID, WwwRedirect: WwwOff}
}

// Check validates the settings; an empty Www redirect means off.
func (s RouteSettings) Check() (RouteSettings, error) {
	switch s.WwwRedirect {
	case "":
		s.WwwRedirect = WwwOff
	case WwwOff, ToApex, ToWww:
	default:
		return s, invalid("www_redirect", "www redirect must be off, to_apex or to_www")
	}
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

// Counterparts maps each counterpart host the Www redirect adds to the
// Domain it redirects to. A counterpart that is one of the Domains itself,
// or not a hostname of at least two labels (www.com → com), is left out.
func Counterparts(domains []string, mode WwwRedirect) map[string]string {
	out := map[string]string{}
	own := make(map[string]bool, len(domains))
	for _, d := range domains {
		own[d] = true
	}
	for _, d := range domains {
		var counterpart string
		switch {
		case mode == ToApex && !strings.HasPrefix(d, "www."):
			counterpart = "www." + d
		case mode == ToWww && strings.HasPrefix(d, "www."):
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
