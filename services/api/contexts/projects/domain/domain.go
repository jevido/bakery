// Package domain is the projects model: Projects, Environments,
// Applications and Environment variables, and the rules for each.
package domain

import (
	"fmt"
	"math"
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

// BuildPack is how an Application becomes an Image.
type BuildPack string

const (
	// Dockerfile builds the Dockerfile at a path in the Source.
	Dockerfile BuildPack = "dockerfile"
	// Nixpacks lets Nixpacks write the Dockerfile for the Source.
	Nixpacks BuildPack = "nixpacks"
	// Static serves the Publish directory of the Source on port 80.
	Static BuildPack = "static"
	// Image pulls the Image reference; there is no Source.
	Image BuildPack = "image"
)

// StaticPort is the port every static Application listens on.
const StaticPort = 80

func (b BuildPack) Valid() bool {
	switch b {
	case Dockerfile, Nixpacks, Static, Image:
		return true
	}
	return false
}

type Application struct {
	ID            uint64
	EnvironmentID uint64
	ProjectID     uint64
	Name          string
	Slug          string
	BuildPack     BuildPack
	// ImageReference is set exactly when BuildPack is Image; the Source
	// fields are then empty.
	ImageReference   string
	PublishDirectory string
	GitURL           string
	GitBranch        string
	DockerfilePath   string
	Port             int
	// Domains are 1 to MaxDomains hostnames; the first is the primary one.
	Domains []string
	// DeployKey is set exactly when the Source is SSH.
	DeployKey DeployKey
	// RegistryCredentials are only set for the image pack. Password is
	// empty unless the Application was read for a Deployment.
	RegistryCredentials RegistryCredentials
	HealthCheck         HealthCheck
	// Storages are the Application's Persistent storages.
	Storages       []Storage
	ResourceLimits ResourceLimits
	// ServerID is the Target server, 0 for the Local server. Set when the
	// Application is created and never changed.
	ServerID uint64
}

// ResourceLimits cap what each Container of an Application may use. Zero
// means unlimited.
type ResourceLimits struct {
	MemoryMB int
	CPUs     float64
}

// Check validates the ranges: memory 16–65536 MB, CPU 0.1–64 cores with at
// most two decimals.
func (l ResourceLimits) Check() error {
	switch {
	case l.MemoryMB != 0 && (l.MemoryMB < 16 || l.MemoryMB > 65536):
		return invalid("resource_limits.memory_mb", "memory must be between 16 and 65536 MB")
	case l.CPUs != 0 && (l.CPUs < 0.1 || l.CPUs > 64):
		return invalid("resource_limits.cpus", "CPU must be between 0.1 and 64 cores")
	case math.Abs(l.CPUs*100-math.Round(l.CPUs*100)) > 1e-6:
		return invalid("resource_limits.cpus", "CPU has at most two decimals")
	}
	return nil
}

// Storage is a Persistent storage: the volume of this name is mounted at
// MountPath in every Container of the Application.
type Storage struct {
	Name      string
	MountPath string
}

// MaxStorages is how many Persistent storages one Application may have.
const MaxStorages = 10

var storageName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// checkStorages validates a whole set: names and mount paths valid and
// each used once.
func checkStorages(storages []Storage) ([]Storage, error) {
	if len(storages) > MaxStorages {
		return nil, invalid("storages", "an application has at most %d persistent storages", MaxStorages)
	}
	out := make([]Storage, len(storages))
	names, paths := map[string]bool{}, map[string]bool{}
	for i, s := range storages {
		s.Name, s.MountPath = strings.TrimSpace(s.Name), strings.TrimSpace(s.MountPath)
		switch {
		case !storageName.MatchString(s.Name):
			return nil, invalid("storages", "%q is not a valid storage name (lowercase letters, digits and -, at most 40)", s.Name)
		case names[s.Name]:
			return nil, invalid("storages", "storage %s is listed twice", s.Name)
		case !strings.HasPrefix(s.MountPath, "/") || path.Clean(s.MountPath) != s.MountPath:
			return nil, invalid("storages", "mount path of %s must be an absolute, clean path like /data", s.Name)
		case s.MountPath == "/":
			return nil, invalid("storages", "mount path of %s cannot be /", s.Name)
		case strings.ContainsAny(s.MountPath, ":,\n\r\x00") || len(s.MountPath) > 200:
			return nil, invalid("storages", "mount path of %s must be at most 200 characters, without : or ,", s.Name)
		case paths[s.MountPath]:
			return nil, invalid("storages", "%s is mounted twice", s.MountPath)
		}
		names[s.Name], paths[s.MountPath] = true, true
		out[i] = s
	}
	return out, nil
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

// RegistryCredentials are what an image Application pulls with. Both
// empty means anonymous.
type RegistryCredentials struct {
	Username string
	Password string
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
	Name             string
	BuildPack        BuildPack
	ImageReference   string
	PublishDirectory string
	GitURL           string
	GitBranch        string
	DockerfilePath   string
	Port             int
	// Domains empty means the one default Domain.
	Domains []string
	// HealthCheck nil keeps the Application's current one (the default
	// for a new Application).
	HealthCheck *HealthCheck
	// Storages nil keeps the current ones (none for a new Application).
	Storages *[]Storage
	// ResourceLimits nil keeps the current ones (none for a new
	// Application).
	ResourceLimits *ResourceLimits
	// RegistryCredentials nil keeps the current ones; an empty Username
	// removes them; a Username with an empty Password keeps the stored
	// password (the service fills it in before Normalize).
	RegistryCredentials *RegistryCredentials
	// ServerID is the Target server (0 the Local server). Only read when
	// the Application is created; an update never changes it.
	ServerID uint64
}

var (
	hostnameLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	nonSlug       = regexp.MustCompile(`[^a-z0-9]+`)
	variableName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
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
	in.ImageReference = strings.TrimSpace(in.ImageReference)
	in.PublishDirectory = strings.TrimSpace(in.PublishDirectory)
	if in.BuildPack == "" {
		in.BuildPack = Dockerfile
	}
	if in.BuildPack == Image {
		in.GitURL, in.GitBranch, in.DockerfilePath = "", "", ""
	} else {
		in.ImageReference = ""
		if in.GitBranch == "" {
			in.GitBranch = "main"
		}
		if in.DockerfilePath == "" {
			in.DockerfilePath = "Dockerfile"
		}
	}
	if in.BuildPack == Static {
		if in.PublishDirectory == "" {
			in.PublishDirectory = "."
		}
		in.Port = StaticPort
	} else {
		in.PublishDirectory = "."
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
	if !in.BuildPack.Valid() {
		return in, invalid("build_pack", "build pack must be dockerfile, nixpacks, static or image")
	}
	if in.BuildPack == Image {
		if err := checkImageReference(in.ImageReference); err != nil {
			return in, err
		}
	} else {
		if err := checkGitURL(in.GitURL); err != nil {
			return in, err
		}
		if strings.ContainsAny(in.GitBranch, " \t\n") || strings.HasPrefix(in.GitBranch, "-") {
			return in, invalid("git_branch", "branch is not a valid git branch name")
		}
		if err := checkRepositoryPath("dockerfile_path", "Dockerfile path", in.DockerfilePath); err != nil {
			return in, err
		}
	}
	if in.RegistryCredentials != nil {
		c := RegistryCredentials{Username: strings.TrimSpace(in.RegistryCredentials.Username), Password: in.RegistryCredentials.Password}
		if c.Username == "" {
			c.Password = ""
		}
		in.RegistryCredentials = &c
	}
	if in.BuildPack != Image {
		in.RegistryCredentials = &RegistryCredentials{}
	} else if c := in.RegistryCredentials; c != nil {
		switch {
		case len(c.Username) > 255 || strings.ContainsAny(c.Username, " \t\n\r"):
			return in, invalid("registry_credentials", "registry username is at most 255 characters without spaces")
		case c.Username != "" && c.Password == "":
			return in, invalid("registry_credentials", "registry password or token is required with a username")
		case len(c.Password) > 4096:
			return in, invalid("registry_credentials", "registry password is at most 4096 characters")
		}
	}
	if in.BuildPack == Static {
		if err := checkRepositoryPath("publish_directory", "publish directory", in.PublishDirectory); err != nil {
			return in, err
		}
	}
	if in.Port < 1 || in.Port > 65535 {
		return in, invalid("port", "port must be between 1 and 65535")
	}
	domains, err := normalizeDomains(in.Domains)
	if err != nil {
		return in, err
	}
	in.Domains = domains
	if l := in.ResourceLimits; l != nil {
		if err := l.Check(); err != nil {
			return in, err
		}
	}
	if in.Storages != nil {
		storages, err := checkStorages(*in.Storages)
		if err != nil {
			return in, err
		}
		in.Storages = &storages
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

// MaxDomains is how many Domains one Application may have.
const MaxDomains = 10

// normalizeDomains lowercases and trims each Domain, drops empty ones and
// checks the list: hostnames, each once, at most MaxDomains.
func normalizeDomains(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		switch {
		case d == "":
			continue
		case !IsHostname(d):
			return nil, invalid("domains", "%s is not a hostname like app.example.com", d)
		case seen[d]:
			return nil, invalid("domains", "%s is listed twice", d)
		}
		seen[d] = true
		out = append(out, d)
	}
	if len(out) > MaxDomains {
		return nil, invalid("domains", "an application has at most %d domains", MaxDomains)
	}
	return out, nil
}

// PrimaryDomain is the first Domain, or "" for none.
func (a Application) PrimaryDomain() string {
	if len(a.Domains) == 0 {
		return ""
	}
	return a.Domains[0]
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

// checkRepositoryPath keeps a path of the Source inside the clone.
func checkRepositoryPath(field, what, p string) error {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "-") {
		return invalid(field, "%s must be relative to the repository root", what)
	}
	if strings.ContainsAny(p, "\n\r") {
		return invalid(field, "%s must be on one line", what)
	}
	for _, part := range strings.Split(path.Clean(p), "/") {
		if part == ".." {
			return invalid(field, "%s must stay inside the repository", what)
		}
	}
	return nil
}

// checkImageReference asks for a reference that names its registry: a
// short name like nginx resolves differently per server (Podman's
// unqualified-search-registries) and may prompt.
func checkImageReference(ref string) error {
	switch {
	case ref == "":
		return invalid("image_reference", "image reference is required")
	case len(ref) > 500:
		return invalid("image_reference", "image reference is at most 500 characters")
	case strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, " \t\n\r\"'`$\\"):
		return invalid("image_reference", "image reference must not contain spaces or quotes")
	}
	host, _, found := strings.Cut(ref, "/")
	if !found || (!strings.ContainsAny(host, ".:") && host != "localhost") {
		return invalid("image_reference", "use a full reference like docker.io/library/nginx:1.27")
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

// EnvironmentVariable is a variable of an Application, or a Shared variable of a
// Project or Environment. Build and Runtime are its scope.
type EnvironmentVariable struct {
	Name    string
	Value   string
	Build   bool
	Runtime bool
}

// CheckEnvironmentVariables validates a whole set: names valid and unique, and every
// variable reaching the build, the Container or both.
func CheckEnvironmentVariables(vars []EnvironmentVariable) error {
	seen := make(map[string]bool, len(vars))
	for _, v := range vars {
		if !variableName.MatchString(v.Name) {
			return invalid("environment_variables", "%q is not a valid variable name (letters, digits and _, not starting with a digit)", v.Name)
		}
		if seen[v.Name] {
			return invalid("environment_variables", "%s is set twice", v.Name)
		}
		if !v.Build && !v.Runtime {
			return invalid("environment_variables", "%s must be available at build time, at runtime or both", v.Name)
		}
		seen[v.Name] = true
	}
	return nil
}

// Variable levels, from the widest to the one that wins.
const (
	FromProject     = "project"
	FromEnvironment = "environment"
	FromApplication = "application"
)

// InheritedVariable is a Shared variable as seen from one Application.
// Overridden means a narrower level sets the same name, so this value is
// not used.
type InheritedVariable struct {
	EnvironmentVariable
	From       string
	Overridden bool
}

// Inherited lists the Shared variables an Application gets, Environment
// ones first.
func Inherited(project, environment, application []EnvironmentVariable) []InheritedVariable {
	names := func(vars []EnvironmentVariable) map[string]bool {
		m := make(map[string]bool, len(vars))
		for _, v := range vars {
			m[v.Name] = true
		}
		return m
	}
	inApp, inEnv := names(application), names(environment)
	out := make([]InheritedVariable, 0, len(project)+len(environment))
	for _, v := range environment {
		out = append(out, InheritedVariable{EnvironmentVariable: v, From: FromEnvironment, Overridden: inApp[v.Name]})
	}
	for _, v := range project {
		out = append(out, InheritedVariable{EnvironmentVariable: v, From: FromProject, Overridden: inApp[v.Name] || inEnv[v.Name]})
	}
	return out
}

// Merge is what an Application's build and Container get: Application
// variables win over Environment ones, which win over Project ones. A name
// set at a narrower level takes that level's scope too.
func Merge(project, environment, application []EnvironmentVariable) (build, runtime map[string]string) {
	merged := map[string]EnvironmentVariable{}
	for _, level := range [][]EnvironmentVariable{project, environment, application} {
		for _, v := range level {
			merged[v.Name] = v
		}
	}
	build, runtime = map[string]string{}, map[string]string{}
	for name, v := range merged {
		if v.Build {
			build[name] = v.Value
		}
		if v.Runtime {
			runtime[name] = v.Value
		}
	}
	return build, runtime
}
