// Package domain is the servers model: the Server, how it is reached, its
// Validation and its Cleanup. It depends on nothing outside the standard
// library.
package domain

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
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

// ErrLocalServer is returned when changing or deleting the Local server.
var ErrLocalServer = errors.New("the local server cannot be changed or deleted")

// Kind says how a Server is reached.
type Kind string

const (
	Local  Kind = "local"
	Remote Kind = "remote"
)

// Status follows the latest Validation.
type Status string

const (
	Unvalidated Status = "unvalidated"
	Reachable   Status = "reachable"
	Unreachable Status = "unreachable"
)

// LocalName is the name of the Local server.
const LocalName = "localhost"

// Check names, in the order a Validation runs them.
const (
	CheckSSH    = "ssh"
	CheckPodman = "podman"
	CheckSocket = "socket"
	CheckLinger = "linger"
	CheckPorts  = "ports"
)

// Check is one outcome of a Validation. A Server is Reachable when every
// Required check is OK.
type Check struct {
	Name     string
	OK       bool
	Required bool
	Detail   string
}

// Validation is the checks run against a Server, and when.
type Validation struct {
	Checks    []Check
	CheckedAt time.Time
}

// Passed reports whether every required check is OK.
func (v Validation) Passed() bool {
	if len(v.Checks) == 0 {
		return false
	}
	for _, c := range v.Checks {
		if c.Required && !c.OK {
			return false
		}
	}
	return true
}

// Cleanup is when a Server was last cleaned up and how many bytes that
// freed.
type Cleanup struct {
	At        time.Time
	Reclaimed int64
}

// ServerKey is the SSH key pair Bakery logs in to a Remote server with. The
// private half is OpenSSH PEM, the public half an authorized_keys line.
type ServerKey struct {
	Public  string
	Private string
}

// Server is a machine Bakery runs Containers on.
type Server struct {
	ID   uint64
	Name string
	Kind Kind
	// Host, Port and User are empty on the Local server.
	Host string
	Port int
	User string
	Key  ServerKey
	// HostKey is the pinned SSH host key (authorized_keys format); empty
	// until the first successful connection.
	HostKey     string
	Status      Status
	Validation  Validation
	LastCleanup Cleanup
	// FailedProbes counts the Server probes in a row that could not reach
	// it; DiskAlmostFull is set by a probe that found its disk almost full.
	FailedProbes   int
	DiskAlmostFull bool
	CreatedAt      time.Time
}

// Input is what the Owner types for a Remote server.
type Input struct {
	Name string
	Host string
	Port int
	User string
}

// DefaultPort is the SSH port used when none is given.
const DefaultPort = 22

var (
	serverName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,62}$`)
	hostName   = regexp.MustCompile(`^([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)
	userName   = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
)

func (in Input) normalize() (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Host = strings.TrimSpace(in.Host)
	in.User = strings.TrimSpace(in.User)
	if in.Port == 0 {
		in.Port = DefaultPort
	}
	switch {
	case in.Name == "":
		return in, invalid("name", "name is required")
	case !serverName.MatchString(in.Name):
		return in, invalid("name", "name is 1–63 letters, digits, spaces, '.', '_' and '-', starting with a letter or digit")
	case strings.EqualFold(in.Name, LocalName):
		return in, invalid("name", "%q is the local server", LocalName)
	case in.Host == "":
		return in, invalid("host", "host is required")
	case net.ParseIP(in.Host) == nil && (len(in.Host) > 253 || !hostName.MatchString(in.Host)):
		return in, invalid("host", "host is a hostname or an IP address, e.g. 203.0.113.10")
	case in.Port < 1 || in.Port > 65535:
		return in, invalid("port", "port must be between 1 and 65535")
	case !userName.MatchString(in.User):
		return in, invalid("user", "user is a Linux user name, e.g. bakery")
	}
	return in, nil
}

// NewLocal is the Local server.
func NewLocal() Server {
	return Server{Name: LocalName, Kind: Local, Status: Unvalidated}
}

// NewRemote validates the input for a Remote server reached with key.
func NewRemote(in Input, key ServerKey) (Server, error) {
	in, err := in.normalize()
	if err != nil {
		return Server{}, err
	}
	if key.Public == "" || key.Private == "" {
		return Server{}, errors.New("servers: a Remote server needs a Server key")
	}
	return Server{Name: in.Name, Kind: Remote, Host: in.Host, Port: in.Port, User: in.User, Key: key, Status: Unvalidated}, nil
}

// Edit changes a Remote server. A new host, port or user is another
// machine or account: the Host key is forgotten and the Server must be
// validated again.
func (s *Server) Edit(in Input) error {
	if s.Kind == Local {
		return ErrLocalServer
	}
	in, err := in.normalize()
	if err != nil {
		return err
	}
	if in.Host != s.Host || in.Port != s.Port || in.User != s.User {
		s.HostKey = ""
		s.Status = Unvalidated
		s.Validation = Validation{}
		s.FailedProbes, s.DiskAlmostFull = 0, false
	}
	s.Name, s.Host, s.Port, s.User = in.Name, in.Host, in.Port, in.User
	return nil
}

// RefID is how other contexts name the Server: 0 for the Local server, so
// "local" has one representation outside this context, else its id.
func (s Server) RefID() uint64 {
	if s.Kind == Local {
		return 0
	}
	return s.ID
}

// CanDelete refuses the Local server.
func (s Server) CanDelete() error {
	if s.Kind == Local {
		return ErrLocalServer
	}
	return nil
}

// ForgetHostKey drops the pinned Host key; the next connection pins the
// one the server presents then.
func (s *Server) ForgetHostKey() error {
	if s.Kind == Local {
		return ErrLocalServer
	}
	s.HostKey = ""
	s.Status = Unvalidated
	return nil
}

// RecordValidation sets the Status from v and pins hostKey (what the server
// presented on a connection that got through) if none is pinned yet.
func (s *Server) RecordValidation(v Validation, hostKey string) {
	s.Validation = v
	s.FailedProbes = 0
	if v.Passed() {
		s.Status = Reachable
	} else {
		s.Status = Unreachable
	}
	if s.Kind == Remote && s.HostKey == "" && hostKey != "" {
		s.HostKey = hostKey
	}
}

// RecordCleanup remembers the latest Cleanup.
func (s *Server) RecordCleanup(c Cleanup) {
	s.LastCleanup = c
}
