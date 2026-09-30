// Package domain is the databases model: the Database, its Engine and the
// rules for each. It depends on nothing outside the standard library.
package domain

import (
	"fmt"
	"math"
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

// DesiredState is what the Owner asked a Database to be.
type DesiredState string

const (
	Running DesiredState = "running"
	Stopped DesiredState = "stopped"
)

// Status is what a Database's Container is doing, read from Podman.
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusExited   Status = "exited"
	StatusMissing  Status = "missing"
)

// Credentials are generated once and never change. RootPassword is only
// used by MySQL and MariaDB.
type Credentials struct {
	Username     string
	Password     string
	RootPassword string
	DatabaseName string
}

// ResourceLimits cap what the Database's Container may use; zero is
// unlimited.
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

// Database is the aggregate root: one Engine running as one Container in an
// Environment.
type Database struct {
	ID            uint64
	EnvironmentID uint64
	ProjectID     uint64
	Name          string
	Slug          string
	Engine        Engine
	Version       string
	Credentials   Credentials
	// PublicPort is the host port it is published on; 0 is none.
	PublicPort     int
	ResourceLimits ResourceLimits
	DesiredState   DesiredState
}

// Input is what the Owner chooses. Engine is only read on creation.
type Input struct {
	Name           string
	Engine         Engine
	Version        string
	PublicPort     int
	ResourceLimits ResourceLimits
}

var versionTag = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

func (in Input) normalize(engine Engine) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Version = strings.TrimSpace(in.Version)
	if in.Version == "" {
		in.Version = engine.Spec().DefaultVersion
	}
	switch {
	case in.Name == "":
		return in, invalid("name", "name is required")
	case len(in.Name) > 100:
		return in, invalid("name", "name is at most 100 characters")
	case !versionTag.MatchString(in.Version):
		return in, invalid("version", "%q is not a valid image tag", in.Version)
	case in.PublicPort != 0 && (in.PublicPort < 1024 || in.PublicPort > 65535):
		return in, invalid("public_port", "public port must be between 1024 and 65535")
	}
	return in, in.ResourceLimits.Check()
}

// NewDatabase validates the input and generates the Credentials. The slug
// must already be free; the Database starts out wanted running.
func NewDatabase(environmentID, projectID uint64, in Input, slug string, generatePassword func() string) (Database, error) {
	if !in.Engine.Valid() {
		return Database{}, invalid("engine", "engine must be one of %s", strings.Join(engineNames(), ", "))
	}
	in, err := in.normalize(in.Engine)
	if err != nil {
		return Database{}, err
	}
	creds := Credentials{Username: DefaultUsername, Password: generatePassword(), DatabaseName: strings.ReplaceAll(slug, "-", "_")}
	if in.Engine.Spec().RootPassword {
		creds.RootPassword = generatePassword()
	}
	return Database{
		EnvironmentID: environmentID, ProjectID: projectID,
		Name: in.Name, Slug: slug, Engine: in.Engine, Version: in.Version,
		Credentials: creds, PublicPort: in.PublicPort, ResourceLimits: in.ResourceLimits,
		DesiredState: Running,
	}, nil
}

// Update changes what may change: name, version, Public port and Resource
// limits. It reports whether the Container has to be recreated for it.
func (d *Database) Update(in Input) (recreate bool, err error) {
	in, err = in.normalize(d.Engine)
	if err != nil {
		return false, err
	}
	recreate = in.Version != d.Version || in.PublicPort != d.PublicPort || in.ResourceLimits != d.ResourceLimits
	d.Name, d.Version, d.PublicPort, d.ResourceLimits = in.Name, in.Version, in.PublicPort, in.ResourceLimits
	return recreate, nil
}

// DefaultUsername is the user every Database is created with.
const DefaultUsername = "bakery"

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a name into the base of a Slug; an empty result falls back
// to the Engine's name.
func Slugify(name string, engine Engine) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		s = string(engine)
	}
	return s
}

// ContainerName is the Database's Container and its hostname on the bakery
// network.
func ContainerName(slug string) string { return "bakery-db-" + slug }

// VolumeName is the volume the Database keeps its data in.
func VolumeName(id uint64) string { return fmt.Sprintf("bakery-db-%d-data", id) }
