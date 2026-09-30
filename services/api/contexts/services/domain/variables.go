package domain

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// MagicKind says whether and how Bakery fills in a Service variable. The
// names follow Coolify's template convention.
type MagicKind string

const (
	MagicNone       MagicKind = ""
	MagicPassword   MagicKind = "password"    // SERVICE_PASSWORD_<X>: 32 characters
	MagicPassword64 MagicKind = "password_64" // SERVICE_PASSWORD_64_<X>: 64 characters
	MagicUser       MagicKind = "user"        // SERVICE_USER_<X>: 16 characters
	MagicBase64     MagicKind = "base64"      // SERVICE_BASE64_<X>: 32 random bytes, base64
	MagicBase64_64  MagicKind = "base64_64"   // SERVICE_BASE64_64_<X>: 64 random bytes, base64
	MagicFQDN       MagicKind = "fqdn"        // SERVICE_FQDN_<NAME>[_<PORT>]: the primary Domain
	MagicURL        MagicKind = "url"         // SERVICE_URL_<NAME>[_<PORT>]: https:// and the primary Domain
)

// Generated reports whether the value is generated once and stored (the
// Domain ones are derived from the Component's Domains instead).
func (k MagicKind) Generated() bool {
	return k != MagicNone && k != MagicFQDN && k != MagicURL
}

// Longest prefix first, so SERVICE_PASSWORD_64_ wins over SERVICE_PASSWORD_.
var magicPrefixes = []struct {
	prefix string
	kind   MagicKind
}{
	{"SERVICE_PASSWORD_64_", MagicPassword64},
	{"SERVICE_PASSWORD_", MagicPassword},
	{"SERVICE_USER_", MagicUser},
	{"SERVICE_BASE64_64_", MagicBase64_64},
	{"SERVICE_BASE64_", MagicBase64},
	{"SERVICE_FQDN_", MagicFQDN},
	{"SERVICE_URL_", MagicURL},
}

// ClassifyVariable says whether name is a Magic variable. subject is the
// part after the prefix (for FQDN and URL: without the port); port is
// only set for SERVICE_FQDN_<NAME>_<PORT> and SERVICE_URL_<NAME>_<PORT>.
func ClassifyVariable(name string) (kind MagicKind, subject string, port int) {
	for _, m := range magicPrefixes {
		rest, ok := strings.CutPrefix(name, m.prefix)
		if !ok || rest == "" {
			continue
		}
		if m.kind == MagicFQDN || m.kind == MagicURL {
			if i := strings.LastIndex(rest, "_"); i > 0 {
				if n, err := strconv.Atoi(rest[i+1:]); err == nil && n >= 1 && n <= 65535 {
					return m.kind, rest[:i], n
				}
			}
		}
		return m.kind, rest, 0
	}
	return MagicNone, "", 0
}

// VariableSubject is how a Component name appears in a Magic variable:
// upper case, - as _.
func VariableSubject(component string) string {
	return strings.ToUpper(strings.ReplaceAll(component, "-", "_"))
}

// VariableRef is one variable a Compose file refers to.
type VariableRef struct {
	Name       string
	Default    string
	HasDefault bool
}

// Variables returns every variable the Compose file refers to, each once,
// in the order first seen.
func Variables(c Compose) []VariableRef {
	var out []VariableRef
	seen := map[string]int{}
	add := func(s string) {
		_ = expand(s, func(name, def string, hasDef bool) (string, bool) {
			if i, ok := seen[name]; ok {
				if hasDef && !out[i].HasDefault {
					out[i].Default, out[i].HasDefault = def, true
				}
			} else {
				seen[name] = len(out)
				out = append(out, VariableRef{Name: name, Default: def, HasDefault: hasDef})
			}
			return "", true
		})
	}
	for _, s := range c.Components {
		for _, v := range componentStrings(s) {
			add(v)
		}
	}
	return out
}

// componentStrings is every field interpolation applies to, environment in
// key order so the result is stable.
func componentStrings(s ComponentSpec) []string {
	out := []string{s.Image, s.WorkingDir, s.User}
	out = append(out, s.Command...)
	out = append(out, s.Entrypoint...)
	for _, k := range slices.Sorted(maps.Keys(s.Environment)) {
		out = append(out, s.Environment[k])
	}
	return out
}

// PublicComponents returns the Components that are public, with their
// port: those whose environment names SERVICE_FQDN_<NAME>_<PORT> or
// SERVICE_URL_<NAME>_<PORT> for themselves, as a key or inside a value.
func PublicComponents(c Compose) (map[string]int, error) {
	out := map[string]int{}
	for _, s := range c.Components {
		var names []string
		for k, v := range s.Environment {
			names = append(names, k)
			_ = expand(v, func(name, _ string, _ bool) (string, bool) {
				names = append(names, name)
				return "", true
			})
		}
		slices.Sort(names)
		for _, name := range names {
			kind, subject, port := ClassifyVariable(name)
			if (kind != MagicFQDN && kind != MagicURL) || port == 0 {
				continue
			}
			if subject != VariableSubject(s.Name) {
				return nil, fmt.Errorf("%s in %s names another Component; put it in the environment of the Component it makes public", name, s.Name)
			}
			if p, ok := out[s.Name]; ok && p != port {
				return nil, fmt.Errorf("%s is public on both port %d and %d; pick one", s.Name, p, port)
			}
			out[s.Name] = port
		}
	}
	return out, nil
}

// UnsetVariableError names the variables Interpolate had no value for.
type UnsetVariableError struct{ Names []string }

func (e *UnsetVariableError) Error() string {
	return "no value for " + strings.Join(e.Names, ", ")
}

// Interpolate resolves every variable with values; a variable without a
// value uses its default, and one without either is an error.
func Interpolate(c Compose, values map[string]string) (Compose, error) {
	var missing []string
	resolve := func(s string) string {
		out := expand(s, func(name, def string, hasDef bool) (string, bool) {
			if v, ok := values[name]; ok {
				return v, true
			}
			if hasDef {
				return def, true
			}
			if !slices.Contains(missing, name) {
				missing = append(missing, name)
			}
			return "", false
		})
		return out
	}
	resolveAll := func(in []string) []string {
		if in == nil {
			return nil
		}
		out := make([]string, len(in))
		for i, s := range in {
			out[i] = resolve(s)
		}
		return out
	}
	out := Compose{Volumes: slices.Clone(c.Volumes)}
	for _, s := range c.Components {
		r := s
		r.Image, r.WorkingDir, r.User = resolve(s.Image), resolve(s.WorkingDir), resolve(s.User)
		r.Command, r.Entrypoint = resolveAll(s.Command), resolveAll(s.Entrypoint)
		r.Environment = make(map[string]string, len(s.Environment))
		for k, v := range s.Environment {
			r.Environment[k] = resolve(v)
		}
		r.Labels = maps.Clone(s.Labels)
		r.Volumes, r.DependsOn, r.Expose = slices.Clone(s.Volumes), slices.Clone(s.DependsOn), slices.Clone(s.Expose)
		out.Components = append(out.Components, r)
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return Compose{}, &UnsetVariableError{Names: missing}
	}
	return out, nil
}

// expand replaces $$, $NAME, ${NAME}, ${NAME:-default}, ${NAME-default}
// and ${NAME:?message} in s. lookup gets the name and default (itself
// expanded); ok false leaves the reference empty. A $ that starts none of
// these stays as it is.
func expand(s string, lookup func(name, def string, hasDef bool) (string, bool)) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		next := s[i+1]
		switch {
		case next == '$':
			b.WriteByte('$')
			i++
		case next == '{':
			end := closingBrace(s, i+2)
			if end < 0 {
				b.WriteString(s[i:])
				return b.String()
			}
			body := s[i+2 : end]
			name, def, hasDef := body, "", false
			if j := strings.IndexAny(body, ":-?"); j >= 0 {
				name = body[:j]
				op := body[j:]
				switch {
				case strings.HasPrefix(op, ":-"):
					def, hasDef = op[2:], true
				case strings.HasPrefix(op, "-"):
					def, hasDef = op[1:], true
				}
				// :? and ? mean "required": no default.
			}
			if !envName.MatchString(name) {
				b.WriteString(s[i : end+1])
				i = end
				continue
			}
			if hasDef {
				def = expand(def, lookup)
			}
			v, _ := lookup(name, def, hasDef)
			b.WriteString(v)
			i = end
		case next == '_' || isLetter(next):
			j := i + 1
			for j < len(s) && (s[j] == '_' || isLetter(s[j]) || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			v, _ := lookup(s[i+1:j], "", false)
			b.WriteString(v)
			i = j - 1
		default:
			b.WriteByte('$')
		}
	}
	return b.String()
}

func closingBrace(s string, from int) int {
	depth := 1
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
