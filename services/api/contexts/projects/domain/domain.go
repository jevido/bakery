// Package domain is the projects model: Projects, Environments,
// Applications and Env vars, and the rules for each.
package domain

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// DefaultEnvironment is created with every Project.
const DefaultEnvironment = "production"

// FieldError is a broken rule on one input field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

type Project struct {
	ID           uint64
	Name         string
	Description  string
	Environments []Environment
}

type Environment struct {
	ID           uint64
	ProjectID    uint64
	Name         string
	Applications []Application
}

// NewProject validates a Project's fields. Its production Environment is
// created alongside it by the repository.
func NewProject(name, description string) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, invalid("name", "name is required")
	}
	if len(name) > 100 {
		return Project{}, invalid("name", "name is at most 100 characters")
	}
	return Project{Name: name, Description: strings.TrimSpace(description)}, nil
}

type Application struct {
	ID             uint64
	EnvironmentID  uint64
	ProjectID      uint64
	Name           string
	Slug           string
	GitURL         string
	GitBranch      string
	DockerfilePath string
	Port           int
	Domain         string
	// DeployKey is set exactly when the Source is SSH.
	DeployKey   DeployKey
	HealthCheck HealthCheck
}

// HealthCheck is how a new Container is probed before it takes traffic:
// Path is requested until it answers 2xx or 3xx. Times are in seconds.
type HealthCheck struct {
	Enabled     bool
	Path        string
	Interval    int
	Timeout     int
	Retries     int
	StartPeriod int
}

// DefaultHealthCheck is the Health check of a new Application: off.
func DefaultHealthCheck() HealthCheck {
	return HealthCheck{Path: "/", Interval: 5, Timeout: 5, Retries: 10}
}

// Normalize fills the defaults of unset fields and checks the ranges.
func (h HealthCheck) Normalize() (HealthCheck, error) {
	d := DefaultHealthCheck()
	h.Path = strings.TrimSpace(h.Path)
	if h.Path == "" {
		h.Path = d.Path
	}
	if h.Interval == 0 {
		h.Interval = d.Interval
	}
	if h.Timeout == 0 {
		h.Timeout = d.Timeout
	}
	if h.Retries == 0 {
		h.Retries = d.Retries
	}
	switch {
	case !strings.HasPrefix(h.Path, "/") || strings.ContainsAny(h.Path, " \t\n'\"\\`$"):
		return h, invalid("health_check.path", "path must start with / and contain no spaces or quotes")
	case len(h.Path) > 200:
		return h, invalid("health_check.path", "path is at most 200 characters")
	case h.Interval < 1 || h.Interval > 300:
		return h, invalid("health_check.interval", "interval must be between 1 and 300 seconds")
	case h.Timeout < 1 || h.Timeout > 60:
		return h, invalid("health_check.timeout", "timeout must be between 1 and 60 seconds")
	case h.Retries < 1 || h.Retries > 100:
		return h, invalid("health_check.retries", "retries must be between 1 and 100")
	case h.StartPeriod < 0 || h.StartPeriod > 600:
		return h, invalid("health_check.start_period", "start period must be between 0 and 600 seconds")
	}
	return h, nil
}

// DeployKey is the SSH key pair Bakery generated for one Application. Public
// is an authorized_keys line; Private is the OpenSSH PEM, empty unless the
// Application was read for a Deployment.
type DeployKey struct {
	Public  string
	Private string
}

// ApplicationInput is what the Owner fills in. Empty optional fields get
// their defaults in Normalize.
type ApplicationInput struct {
	Name           string
	GitURL         string
	GitBranch      string
	DockerfilePath string
	Port           int
	Domain         string
	// HealthCheck nil keeps the Application's current one (the default
	// for a new Application).
	HealthCheck *HealthCheck
}

var (
	hostnameLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	nonSlug       = regexp.MustCompile(`[^a-z0-9]+`)
	envVarName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// scpLikeURL is git's user@host:path form (git@github.com:owner/repo.git).
	scpLikeURL = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9][A-Za-z0-9.-]*:[^-\s][^\s]*$`)
)

// Normalize trims the input, fills defaults and checks every rule except
// uniqueness, which only the repository can check.
func (in ApplicationInput) Normalize() (ApplicationInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.GitURL = strings.TrimSpace(in.GitURL)
	in.GitBranch = strings.TrimSpace(in.GitBranch)
	in.DockerfilePath = strings.TrimSpace(in.DockerfilePath)
	in.Domain = strings.ToLower(strings.TrimSpace(in.Domain))
	if in.GitBranch == "" {
		in.GitBranch = "main"
	}
	if in.DockerfilePath == "" {
		in.DockerfilePath = "Dockerfile"
	}

	if in.Name == "" {
		return in, invalid("name", "name is required")
	}
	if len(in.Name) > 100 {
		return in, invalid("name", "name is at most 100 characters")
	}
	if Slugify(in.Name) == "" {
		return in, invalid("name", "name needs at least one letter or digit")
	}
	if err := checkGitURL(in.GitURL); err != nil {
		return in, err
	}
	if strings.ContainsAny(in.GitBranch, " \t\n") || strings.HasPrefix(in.GitBranch, "-") {
		return in, invalid("git_branch", "branch is not a valid git branch name")
	}
	if err := checkDockerfilePath(in.DockerfilePath); err != nil {
		return in, err
	}
	if in.Port < 1 || in.Port > 65535 {
		return in, invalid("port", "port must be between 1 and 65535")
	}
	if in.Domain != "" && !IsHostname(in.Domain) {
		return in, invalid("domain", "domain must be a hostname like app.example.com")
	}
	if in.HealthCheck != nil {
		h, err := in.HealthCheck.Normalize()
		if err != nil {
			return in, err
		}
		in.HealthCheck = &h
	}
	return in, nil
}

// checkGitURL allows https URLs without credentials and SSH URLs, nothing
// else: the deployment worker clones where the API runs, and a file:// URL,
// an ext:: transport or a local path would let an Application read or run
// things there.
func checkGitURL(raw string) error {
	if raw == "" {
		return invalid("git_url", "git URL is required")
	}
	if IsSSHSource(raw) {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return invalid("git_url", "git URL must be https://… without credentials, or an SSH URL like git@host:owner/repo.git")
	}
	return nil
}

// IsSSHSource reports whether raw is an SSH git URL: ssh://user@host[:port]/path
// or user@host:path. The user is required and no part may start with "-", so
// neither can be read as an ssh option.
func IsSSHSource(raw string) bool {
	if strings.HasPrefix(raw, "ssh://") {
		u, err := url.Parse(raw)
		if err != nil || u.User == nil || u.User.Username() == "" || strings.HasPrefix(u.User.Username(), "-") {
			return false
		}
		if _, hasPassword := u.User.Password(); hasPassword {
			return false
		}
		host := u.Hostname()
		return host != "" && !strings.HasPrefix(host, "-") && len(strings.Trim(u.Path, "/")) > 0 && !strings.ContainsAny(raw, " \t\n")
	}
	return !strings.HasPrefix(raw, "-") && scpLikeURL.MatchString(raw)
}

func checkDockerfilePath(p string) error {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "-") {
		return invalid("dockerfile_path", "Dockerfile path must be relative to the repository root")
	}
	for _, part := range strings.Split(path.Clean(p), "/") {
		if part == ".." {
			return invalid("dockerfile_path", "Dockerfile path must stay inside the repository")
		}
	}
	return nil
}

// IsHostname reports whether s is a lowercase DNS hostname with at least two
// labels.
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

// Slugify turns a name into a slug: lowercase, [a-z0-9-], at most 40
// characters.
func Slugify(name string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	return s
}

// DefaultDomain is the Domain an Application gets when none is given.
func DefaultDomain(slug, suffix string) string {
	return slug + "." + suffix
}

type EnvVar struct {
	Name  string
	Value string
}

// CheckEnvVars validates a whole set: names valid and unique.
func CheckEnvVars(vars []EnvVar) error {
	seen := make(map[string]bool, len(vars))
	for _, v := range vars {
		if !envVarName.MatchString(v.Name) {
			return invalid("env", "%q is not a valid env var name (letters, digits and _, not starting with a digit)", v.Name)
		}
		if seen[v.Name] {
			return invalid("env", "%s is set twice", v.Name)
		}
		seen[v.Name] = true
	}
	return nil
}
