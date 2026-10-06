package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
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

// DesiredState is what a Member asked a Service to be.
type DesiredState string

const (
	Running DesiredState = "running"
	Stopped DesiredState = "stopped"
)

// MaxDomains is how many Domains one Public Component may have.
const MaxDomains = 10

// Component is one entry of the Compose file as the Service keeps it:
// public ones have a port and their Domains, the primary first.
type Component struct {
	Name    string
	Image   string // as written, before interpolation
	Public  bool
	Port    int
	Domains []string
}

// PrimaryDomain is the first Domain, or "" for none.
func (c Component) PrimaryDomain() string {
	if len(c.Domains) == 0 {
		return ""
	}
	return c.Domains[0]
}

// Variable is a Service variable. Magic ones (Generated kinds only) are
// filled in once by Bakery; a Member sets the others, and an empty Value
// with a Default means the Default.
type Variable struct {
	Name       string
	Value      string
	Magic      MagicKind
	Default    string
	HasDefault bool
}

// Service is the aggregate root: a Compose file run as Components in one
// Environment.
type Service struct {
	ID            uint64
	EnvironmentID uint64
	ProjectID     uint64
	Name          string
	// Description is free text of at most 255 characters, empty for none.
	Description string
	Slug        string
	ComposeFile string
	// TemplateKey is the Service template it was created from, or "".
	TemplateKey  string
	DesiredState DesiredState
	Components   []Component
	Variables    []Variable
	// LastError is why the last action failed; "" when it did not.
	LastError string
}

// Generate makes a new value for a generated Magic variable.
type Generate func(MagicKind) string

func checkName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return name, invalid("name", "name is required")
	case len(name) > 100:
		return name, invalid("name", "name is at most 100 characters")
	}
	return name, nil
}

func checkDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > 255 {
		return description, invalid("description", "description is at most 255 characters")
	}
	return description, nil
}

// NewService parses the Compose file, gives each Public Component its
// default Domain below domainSuffix and generates the Magic variables. The
// slug must already be free; the Service starts out wanted running.
func NewService(environmentID, projectID uint64, name, slug, compose, domainSuffix string, generate Generate) (Service, error) {
	name, err := checkName(name)
	if err != nil {
		return Service{}, err
	}
	s := Service{EnvironmentID: environmentID, ProjectID: projectID, Name: name, Slug: slug, DesiredState: Running}
	if err := s.ChangeCompose(compose, domainSuffix, generate); err != nil {
		return Service{}, err
	}
	return s, nil
}

// Rename changes the name.
func (s *Service) Rename(name string) error {
	name, err := checkName(name)
	if err != nil {
		return err
	}
	s.Name = name
	return nil
}

// Describe changes the description.
func (s *Service) Describe(description string) error {
	description, err := checkDescription(description)
	if err != nil {
		return err
	}
	s.Description = description
	return nil
}

// composeError turns a parse error into a ComposeFileError, and any other
// Compose file problem into a FieldError on compose.
func composeError(err error) error {
	var errs ComposeErrors
	if errors.As(err, &errs) {
		return &ComposeFileError{Errors: errs}
	}
	return invalid("compose", "%v", err)
}

// ComposeFileError is a Compose file the Service cannot take.
type ComposeFileError struct{ Errors ComposeErrors }

func (e *ComposeFileError) Error() string { return "compose: " + e.Errors.Error() }

// ChangeCompose replaces the Compose file. Components keep their Domains by
// name, Service variables their values; new Public Components get a
// default Domain and new Magic variables are generated. Variables the file
// no longer refers to are dropped.
func (s *Service) ChangeCompose(src, domainSuffix string, generate Generate) error {
	c, err := ParseCompose(src)
	if err != nil {
		return composeError(err)
	}
	public, err := PublicComponents(c)
	if err != nil {
		return composeError(err)
	}
	old := map[string]Component{}
	for _, comp := range s.Components {
		old[comp.Name] = comp
	}
	components := make([]Component, 0, len(c.Components))
	first := true
	for _, spec := range c.Components {
		comp := Component{Name: spec.Name, Image: spec.Image}
		if port, ok := public[spec.Name]; ok {
			comp.Public, comp.Port = true, port
			if prev, ok := old[spec.Name]; ok && len(prev.Domains) > 0 {
				comp.Domains = slices.Clone(prev.Domains)
			} else {
				comp.Domains = []string{DefaultDomain(s.Slug, spec.Name, first, domainSuffix)}
			}
			first = false
		}
		components = append(components, comp)
	}

	values := map[string]Variable{}
	for _, v := range s.Variables {
		values[v.Name] = v
	}
	var variables []Variable
	for _, ref := range Variables(c) {
		kind, subject, _ := ClassifyVariable(ref.Name)
		if kind == MagicFQDN || kind == MagicURL {
			if !slices.ContainsFunc(components, func(comp Component) bool { return comp.Public && VariableSubject(comp.Name) == subject }) {
				return invalid("compose", "%s names no Public Component; make one public with SERVICE_FQDN_%s_<PORT> in its environment", ref.Name, subject)
			}
			continue // derived from the Component's Domains
		}
		v := Variable{Name: ref.Name, Magic: kind, Default: ref.Default, HasDefault: ref.HasDefault}
		if prev, ok := values[ref.Name]; ok {
			v.Value = prev.Value
		} else if kind.Generated() {
			v.Value = generate(kind)
		}
		variables = append(variables, v)
	}
	s.ComposeFile, s.Components, s.Variables = src, components, variables
	return s.checkDomains()
}

var nonHostname = regexp.MustCompile(`[^a-z0-9-]+`)

// DefaultDomain is the Domain a Public Component gets: <slug>.<suffix> for
// the Service's first, <slug>-<component>.<suffix> for the others.
func DefaultDomain(slug, component string, first bool, suffix string) string {
	if first {
		return slug + "." + suffix
	}
	label := strings.Trim(nonHostname.ReplaceAllString(component, "-"), "-")
	return slug + "-" + label + "." + suffix
}

// Component returns the Component with this name.
func (s Service) Component(name string) (Component, bool) {
	for _, c := range s.Components {
		if c.Name == name {
			return c, true
		}
	}
	return Component{}, false
}

// Domains is every Domain of the Service.
func (s Service) Domains() []string {
	var out []string
	for _, c := range s.Components {
		out = append(out, c.Domains...)
	}
	return out
}

// SetDomains replaces a Public Component's Domains.
func (s *Service) SetDomains(component string, domains []string) error {
	i := slices.IndexFunc(s.Components, func(c Component) bool { return c.Name == component })
	switch {
	case i < 0:
		return invalid("domains", "%s is not a Component of this Service", component)
	case !s.Components[i].Public:
		return invalid("domains", "%s is not public; give it SERVICE_FQDN_%s_<PORT> in the compose file", component, VariableSubject(component))
	}
	clean := make([]string, 0, len(domains))
	for _, d := range domains {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			clean = append(clean, d)
		}
	}
	if len(clean) == 0 {
		return invalid("domains", "%s needs at least one domain", component)
	}
	prev := s.Components[i].Domains
	s.Components[i].Domains = clean
	if err := s.checkDomains(); err != nil {
		s.Components[i].Domains = prev
		return err
	}
	return nil
}

func (s Service) checkDomains() error {
	seen := map[string]bool{}
	for _, c := range s.Components {
		if len(c.Domains) > MaxDomains {
			return invalid("domains", "%s has more than %d domains", c.Name, MaxDomains)
		}
		for _, d := range c.Domains {
			switch {
			case !IsHostname(d):
				return invalid("domains", "%s is not a hostname like app.example.com", d)
			case seen[d]:
				return invalid("domains", "%s is listed twice", d)
			}
			seen[d] = true
		}
	}
	return nil
}

// SetVariable sets a Service variable a Member owns; Magic ones are
// Bakery's.
func (s *Service) SetVariable(name, value string) error {
	i := slices.IndexFunc(s.Variables, func(v Variable) bool { return v.Name == name })
	switch {
	case i < 0:
		return invalid("variables", "the compose file does not use %s", name)
	case s.Variables[i].Magic != MagicNone:
		return invalid("variables", "%s is generated by The Bakery", name)
	case strings.ContainsRune(value, 0):
		return invalid("variables", "%s contains a NUL byte", name)
	}
	s.Variables[i].Value = value
	return nil
}

// Values is every variable's value for Interpolate: stored ones (an unset
// variable a Member set with a default is left out, so the default applies) and
// SERVICE_FQDN_*/SERVICE_URL_* from the Public Components' primary Domains.
func (s Service) Values() map[string]string {
	out := map[string]string{}
	for _, v := range s.Variables {
		if v.Value != "" || !v.HasDefault {
			out[v.Name] = v.Value
		}
	}
	for _, c := range s.Components {
		if !c.Public {
			continue
		}
		subject := VariableSubject(c.Name)
		domain := c.PrimaryDomain()
		for _, suffix := range []string{"", fmt.Sprintf("_%d", c.Port)} {
			out["SERVICE_FQDN_"+subject+suffix] = domain
			out["SERVICE_URL_"+subject+suffix] = "https://" + domain
		}
	}
	return out
}

// Resolved is the Compose file with every variable filled in.
func (s Service) Resolved() (Compose, error) {
	c, err := ParseCompose(s.ComposeFile)
	if err != nil {
		return Compose{}, composeError(err)
	}
	return Interpolate(c, s.Values())
}

var hostnameLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// IsHostname reports whether s is a lowercase DNS hostname with at least
// two labels (the same rule projects applies to Application Domains).
func IsHostname(s string) bool {
	if len(s) > 253 {
		return false
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if !hostnameLabel.MatchString(l) {
			return false
		}
	}
	return true
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a name into the base of a Slug: lowercase [a-z0-9-], at
// most 40 characters, "service" when nothing is left.
func Slugify(name string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		s = "service"
	}
	return s
}

// GeneratedName is the name a Service created from a pasted Compose file
// without a name gets, as Coolify names it: "docker-compose-<random>".
func GeneratedName(random string) string { return "docker-compose-" + random }

// ContainerName is a Component's Container, e.g. bakery-svc-3-web.
func ContainerName(serviceID uint64, component string) string {
	return fmt.Sprintf("bakery-svc-%d-%s", serviceID, component)
}

// NetworkName is the Service network.
func NetworkName(serviceID uint64) string { return fmt.Sprintf("bakery-svc-%d", serviceID) }

// VolumeName is a Service volume.
func VolumeName(serviceID uint64, volume string) string {
	return fmt.Sprintf("bakery-svc-%d-%s", serviceID, volume)
}

// Status is what a Component's Container is doing, read from Podman.
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusExited   Status = "exited"
	StatusMissing  Status = "missing"
)

// ServiceStatus sums up a Service.
type ServiceStatus string

const (
	ServiceRunning   ServiceStatus = "running"
	ServiceStopped   ServiceStatus = "stopped"
	ServiceDeploying ServiceStatus = "deploying"
	ServiceDegraded  ServiceStatus = "degraded"
	ServiceFailed    ServiceStatus = "failed"
)

// Summarize gives the Service status from its Components' statuses, whether
// an action is running and the last error.
func (s Service) Summarize(statuses map[string]Status, busy bool) ServiceStatus {
	if busy {
		return ServiceDeploying
	}
	if s.LastError != "" {
		return ServiceFailed
	}
	running, stopped, starting := 0, 0, 0
	for _, c := range s.Components {
		switch statuses[c.Name] {
		case StatusRunning:
			running++
		case StatusStarting:
			starting++
		case StatusStopped, StatusMissing:
			stopped++
		}
	}
	switch {
	case running == len(s.Components):
		return ServiceRunning
	case starting > 0 && running+starting == len(s.Components):
		return ServiceDeploying
	case s.DesiredState == Stopped && running == 0:
		return ServiceStopped
	case running == 0 && stopped == len(s.Components):
		return ServiceStopped
	}
	return ServiceDegraded
}
