// Package domain is the services model: the Service, its Compose file and
// Components, its Service variables and the rules for each. It depends on
// nothing outside the standard library except go.yaml.in/yaml/v3, a pure
// YAML parser.
package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// MaxComponents is how many Components one Compose file may have.
const MaxComponents = 20

// Compose is a parsed Compose file in the subset Bakery supports.
type Compose struct {
	Components []ComponentSpec
	// Volumes are the names declared under the top-level volumes key.
	Volumes []string
}

// ComponentSpec is one entry under services:, as written (Interpolate
// resolves its variables).
type ComponentSpec struct {
	Name       string
	Image      string
	Command    []string
	Entrypoint []string
	// Environment is the Container's environment. A key listed without a
	// value stands for ${KEY}.
	Environment map[string]string
	Volumes     []VolumeMount
	DependsOn   []string
	WorkingDir  string
	User        string
	Expose      []int
	Labels      map[string]string
}

// VolumeMount mounts a named volume of the Service at Path.
type VolumeMount struct {
	Volume   string
	Path     string
	ReadOnly bool
}

// Component returns the Component with this name.
func (c Compose) Component(name string) (ComponentSpec, bool) {
	for _, s := range c.Components {
		if s.Name == name {
			return s, true
		}
	}
	return ComponentSpec{}, false
}

// VolumeNames returns every named volume a Component mounts or the file
// declares, each once, in the order first seen.
func (c Compose) VolumeNames() []string {
	var out []string
	for _, v := range c.Volumes {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	for _, s := range c.Components {
		for _, m := range s.Volumes {
			if !slices.Contains(out, m.Volume) {
				out = append(out, m.Volume)
			}
		}
	}
	return out
}

// ComposeError is one thing in a Compose file Bakery does not accept.
type ComposeError struct {
	Line    int
	Path    string // e.g. services.app.build
	Message string
}

func (e *ComposeError) Error() string {
	var b strings.Builder
	if e.Line > 0 {
		fmt.Fprintf(&b, "line %d: ", e.Line)
	}
	if e.Path != "" {
		b.WriteString(e.Path + ": ")
	}
	b.WriteString(e.Message)
	return b.String()
}

// ComposeErrors is every ComposeError in one file, in file order.
type ComposeErrors []*ComposeError

func (es ComposeErrors) Error() string {
	msgs := make([]string, len(es))
	for i, e := range es {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// Messages returns each error as one line.
func (es ComposeErrors) Messages() []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Error()
	}
	return out
}

var (
	componentName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)
	volumeName    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
	envName       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Keys a Component may not have, with the reason given to the Owner.
var refusedKeys = map[string]string{
	"build":          "building images is not supported; use image:",
	"ports":          "host ports are not supported; a Component is reached through its Domains (SERVICE_FQDN_<NAME>_<PORT>)",
	"network_mode":   "only the Service's own network is supported",
	"privileged":     "is not allowed",
	"cap_add":        "is not allowed",
	"devices":        "is not allowed",
	"pid":            "is not allowed",
	"ipc":            "is not allowed",
	"extends":        "is not supported",
	"secrets":        "is not supported",
	"configs":        "is not supported",
	"profiles":       "is not supported",
	"env_file":       "is not supported; use environment:",
	"container_name": "is not supported; The Bakery names Containers itself",
	"tmpfs":          "is not supported",
}

// Keys a Component may have that Bakery does not act on.
var ignoredKeys = []string{"restart", "healthcheck", "stop_grace_period", "hostname"}

type parser struct {
	errs ComposeErrors
}

func (p *parser) fail(n *yaml.Node, path, format string, args ...any) {
	line := 0
	if n != nil {
		line = n.Line
	}
	p.errs = append(p.errs, &ComposeError{Line: line, Path: path, Message: fmt.Sprintf(format, args...)})
}

// ParseCompose parses a Compose file. Everything outside the supported
// subset is refused with its line; the error is then ComposeErrors.
func ParseCompose(src string) (Compose, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		return Compose{}, ComposeErrors{{Message: "not valid YAML: " + strings.TrimPrefix(err.Error(), "yaml: ")}}
	}
	if len(doc.Content) == 0 {
		return Compose{}, ComposeErrors{{Message: "the compose file is empty"}}
	}
	root := doc.Content[0]
	p := &parser{}
	if root.Kind != yaml.MappingNode {
		p.fail(root, "", "the compose file must be a mapping with services:")
		return Compose{}, p.errs
	}
	var c Compose
	var services *yaml.Node
	for k, v := range pairs(root) {
		switch key := k.Value; {
		case key == "services":
			services = v
		case key == "volumes":
			c.Volumes = p.topVolumes(v)
		case key == "name" || key == "version" || strings.HasPrefix(key, "x-"):
		case key == "networks":
			p.fail(k, key, "only the Service's own network is supported")
		case key == "secrets" || key == "configs":
			p.fail(k, key, "is not supported")
		default:
			p.fail(k, key, "unknown key")
		}
	}
	switch {
	case services == nil:
		p.fail(root, "services", "is required")
	case services.Kind != yaml.MappingNode || len(services.Content) == 0:
		p.fail(services, "services", "must list at least one Component")
	case len(services.Content)/2 > MaxComponents:
		p.fail(services, "services", "at most %d Components", MaxComponents)
	default:
		for k, v := range pairs(services) {
			if s, ok := p.component(k, v); ok {
				c.Components = append(c.Components, s)
			}
		}
	}
	if len(p.errs) == 0 {
		p.checkDependencies(services, c)
	}
	if len(p.errs) > 0 {
		return Compose{}, p.errs
	}
	return c, nil
}

// pairs yields the key and value nodes of a mapping.
func pairs(m *yaml.Node) func(yield func(k, v *yaml.Node) bool) {
	return func(yield func(k, v *yaml.Node) bool) {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if !yield(m.Content[i], m.Content[i+1]) {
				return
			}
		}
	}
}

func (p *parser) topVolumes(v *yaml.Node) []string {
	if v.Kind != yaml.MappingNode {
		if v.Tag != "!!null" {
			p.fail(v, "volumes", "must be a mapping of volume names")
		}
		return nil
	}
	var out []string
	for k, spec := range pairs(v) {
		if !volumeName.MatchString(k.Value) {
			p.fail(k, "volumes."+k.Value, "not a valid volume name")
			continue
		}
		if spec.Kind == yaml.MappingNode {
			for sk := range pairs(spec) {
				if sk.Value != "name" && !strings.HasPrefix(sk.Value, "x-") {
					p.fail(sk, "volumes."+k.Value+"."+sk.Value, "is not supported")
				}
			}
		}
		out = append(out, k.Value)
	}
	return out
}

func (p *parser) component(k, v *yaml.Node) (ComponentSpec, bool) {
	path := "services." + k.Value
	before := len(p.errs)
	if !componentName.MatchString(k.Value) {
		p.fail(k, path, "a Component name is lowercase letters, digits, - and _, at most 63 characters")
	}
	s := ComponentSpec{Name: k.Value, Environment: map[string]string{}, Labels: map[string]string{}}
	if v.Kind != yaml.MappingNode {
		p.fail(v, path, "must be a mapping")
		return s, false
	}
	for fk, fv := range pairs(v) {
		key, fpath := fk.Value, path+"."+fk.Value
		switch {
		case key == "image":
			s.Image = p.scalar(fv, fpath)
		case key == "command":
			s.Command = p.commandLine(fv, fpath)
		case key == "entrypoint":
			s.Entrypoint = p.commandLine(fv, fpath)
		case key == "environment":
			s.Environment = p.keyValues(fv, fpath, true)
		case key == "labels":
			s.Labels = p.keyValues(fv, fpath, false)
		case key == "volumes":
			s.Volumes = p.mounts(fv, fpath)
		case key == "depends_on":
			s.DependsOn = p.dependsOn(fv, fpath)
		case key == "working_dir":
			s.WorkingDir = p.scalar(fv, fpath)
		case key == "user":
			s.User = p.scalar(fv, fpath)
		case key == "expose":
			s.Expose = p.expose(fv, fpath)
		case key == "networks":
			p.networks(fv, fpath)
		case slices.Contains(ignoredKeys, key) || strings.HasPrefix(key, "x-"):
		case refusedKeys[key] != "":
			p.fail(fk, fpath, "%s", refusedKeys[key])
		default:
			p.fail(fk, fpath, "unknown key")
		}
	}
	if s.Image == "" && len(p.errs) == before {
		p.fail(k, path+".image", "is required")
	}
	return s, len(p.errs) == before
}

func (p *parser) scalar(n *yaml.Node, path string) string {
	if n.Kind != yaml.ScalarNode {
		p.fail(n, path, "must be a single value")
		return ""
	}
	return n.Value
}

// commandLine takes a list, or a string split like a shell would split
// words (quotes and backslashes, no expansion).
func (p *parser) commandLine(n *yaml.Node, path string) []string {
	switch n.Kind {
	case yaml.ScalarNode:
		words, err := splitWords(n.Value)
		if err != nil {
			p.fail(n, path, "%v", err)
		}
		return words
	case yaml.SequenceNode:
		out := make([]string, 0, len(n.Content))
		for _, item := range n.Content {
			out = append(out, p.scalar(item, path))
		}
		return out
	}
	p.fail(n, path, "must be a string or a list")
	return nil
}

func splitWords(s string) ([]string, error) {
	var out []string
	var word strings.Builder
	inWord := false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, inWord = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if inWord {
				out = append(out, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inWord {
		out = append(out, word.String())
	}
	return out, nil
}

// keyValues reads a mapping or a KEY=VALUE list. For environment, a key
// without a value stands for ${KEY}, so it becomes a Service variable.
func (p *parser) keyValues(n *yaml.Node, path string, env bool) map[string]string {
	out := map[string]string{}
	set := func(at *yaml.Node, key, value string, hasValue bool) {
		if env && !envName.MatchString(key) {
			p.fail(at, path, "%q is not a valid variable name", key)
			return
		}
		if env && !hasValue {
			value = "${" + key + "}"
		}
		out[key] = value
	}
	switch n.Kind {
	case yaml.MappingNode:
		for k, v := range pairs(n) {
			if v.Kind != yaml.ScalarNode {
				p.fail(v, path+"."+k.Value, "must be a single value")
				continue
			}
			set(k, k.Value, v.Value, v.Tag != "!!null")
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if item.Kind != yaml.ScalarNode {
				p.fail(item, path, "must be KEY=VALUE")
				continue
			}
			key, value, found := strings.Cut(item.Value, "=")
			set(item, key, value, found)
		}
	default:
		if n.Tag != "!!null" {
			p.fail(n, path, "must be a mapping or a list of KEY=VALUE")
		}
	}
	return out
}

func (p *parser) mounts(n *yaml.Node, path string) []VolumeMount {
	if n.Kind != yaml.SequenceNode {
		p.fail(n, path, "must be a list")
		return nil
	}
	var out []VolumeMount
	for _, item := range n.Content {
		var m VolumeMount
		var ok bool
		switch item.Kind {
		case yaml.ScalarNode:
			m, ok = p.shortMount(item, path)
		case yaml.MappingNode:
			m, ok = p.longMount(item, path)
		default:
			p.fail(item, path, "must be name:/path or a mapping")
		}
		if ok {
			out = append(out, m)
		}
	}
	return out
}

func (p *parser) shortMount(n *yaml.Node, path string) (VolumeMount, bool) {
	parts := strings.Split(n.Value, ":")
	if len(parts) == 1 {
		p.fail(n, path, "%q: anonymous volumes are not supported; name the volume (name:%s)", n.Value, n.Value)
		return VolumeMount{}, false
	}
	if len(parts) > 3 {
		p.fail(n, path, "%q is not name:/path[:ro]", n.Value)
		return VolumeMount{}, false
	}
	m := VolumeMount{Volume: parts[0], Path: parts[1]}
	if len(parts) == 3 {
		for opt := range strings.SplitSeq(parts[2], ",") {
			switch opt {
			case "ro":
				m.ReadOnly = true
			case "rw", "z", "Z":
			default:
				p.fail(n, path, "%q: unknown volume option %q", n.Value, opt)
				return VolumeMount{}, false
			}
		}
	}
	return m, p.checkMount(n, path, m)
}

func (p *parser) longMount(n *yaml.Node, path string) (VolumeMount, bool) {
	var m VolumeMount
	kind := ""
	for k, v := range pairs(n) {
		switch k.Value {
		case "type":
			kind = v.Value
		case "source":
			m.Volume = v.Value
		case "target":
			m.Path = v.Value
		case "read_only":
			m.ReadOnly = v.Value == "true"
		case "volume":
		default:
			p.fail(k, path+"."+k.Value, "is not supported")
			return m, false
		}
	}
	if kind != "volume" {
		p.fail(n, path, "only type: volume is supported, not bind mounts or tmpfs")
		return m, false
	}
	if m.Volume == "" {
		p.fail(n, path, "anonymous volumes are not supported; give the volume a source name")
		return m, false
	}
	return m, p.checkMount(n, path, m)
}

func (p *parser) checkMount(n *yaml.Node, path string, m VolumeMount) bool {
	switch {
	case strings.HasPrefix(m.Volume, ".") || strings.HasPrefix(m.Volume, "/") || strings.HasPrefix(m.Volume, "~") || strings.Contains(m.Volume, "/"):
		p.fail(n, path, "%q is a bind mount; only named volumes are supported", m.Volume)
	case !volumeName.MatchString(m.Volume):
		p.fail(n, path, "%q is not a valid volume name", m.Volume)
	case !strings.HasPrefix(m.Path, "/"):
		p.fail(n, path, "the mount path %q must be absolute", m.Path)
	default:
		return true
	}
	return false
}

func (p *parser) dependsOn(n *yaml.Node, path string) []string {
	var out []string
	switch n.Kind {
	case yaml.SequenceNode:
		for _, item := range n.Content {
			out = append(out, p.scalar(item, path))
		}
	case yaml.MappingNode:
		for k := range pairs(n) {
			out = append(out, k.Value)
		}
	default:
		p.fail(n, path, "must be a list or a mapping")
	}
	return out
}

func (p *parser) expose(n *yaml.Node, path string) []int {
	if n.Kind != yaml.SequenceNode {
		p.fail(n, path, "must be a list of ports")
		return nil
	}
	var out []int
	for _, item := range n.Content {
		v, _, _ := strings.Cut(item.Value, "/")
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			p.fail(item, path, "%q is not a port", item.Value)
			continue
		}
		out = append(out, port)
	}
	return out
}

// networks accepts only the default network, which is the Service network.
func (p *parser) networks(n *yaml.Node, path string) {
	var names []*yaml.Node
	switch n.Kind {
	case yaml.SequenceNode:
		names = n.Content
	case yaml.MappingNode:
		for k := range pairs(n) {
			names = append(names, k)
		}
	}
	for _, name := range names {
		if name.Value != "default" {
			p.fail(name, path, "only the Service's own network is supported")
		}
	}
}

func (p *parser) checkDependencies(services *yaml.Node, c Compose) {
	for _, s := range c.Components {
		for _, d := range s.DependsOn {
			if _, ok := c.Component(d); !ok {
				p.fail(services, "services."+s.Name+".depends_on", "%q is not a Component of this file", d)
			}
		}
	}
	if len(p.errs) == 0 {
		if _, err := StartOrder(c); err != nil {
			p.fail(services, "services", "%v", err)
		}
	}
}

// StartOrder returns the Component names so that each comes after what it
// depends on, otherwise in file order.
func StartOrder(c Compose) ([]string, error) {
	done := map[string]bool{}
	var out []string
	for len(out) < len(c.Components) {
		progressed := false
		for _, s := range c.Components {
			if done[s.Name] {
				continue
			}
			ready := true
			for _, d := range s.DependsOn {
				if _, ok := c.Component(d); !ok {
					return nil, fmt.Errorf("%s depends on %q, which is not a Component", s.Name, d)
				}
				ready = ready && done[d]
			}
			if ready {
				done[s.Name], progressed = true, true
				out = append(out, s.Name)
			}
		}
		if !progressed {
			var left []string
			for _, s := range c.Components {
				if !done[s.Name] {
					left = append(left, s.Name)
				}
			}
			return nil, fmt.Errorf("depends_on goes round in a circle between %s", strings.Join(left, ", "))
		}
	}
	return out, nil
}
