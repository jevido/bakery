// Package domain is the agents context's model: a Guild's Agents, their
// Jobs, Managers and Agent status. It depends on nothing outside the
// standard library.
package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
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

// StatusError refuses a change the Agent's status does not allow.
type StatusError struct {
	Status Status
	Action string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("a %s agent cannot be %s", strings.ReplaceAll(string(e.Status), "_", " "), e.Action)
}

const (
	MaxName         = 100
	MaxTitle        = 200
	MaxCapabilities = 20000
	// MaxChain is how far up a Manager's Chain of command is walked when
	// looking for a cycle, as Paperclip's getChainOfCommand.
	MaxChain = 50
	// MinHeartbeatInterval and MaxHeartbeatInterval bound a Heartbeat
	// policy's interval, in seconds: a minute to a day.
	MinHeartbeatInterval = 60
	MaxHeartbeatInterval = 86400
)

// HeartbeatPolicy is when an Agent wakes by itself (Paperclip's
// runtimeConfig.heartbeat, trimmed): a timer Run every IntervalSec seconds
// while Enabled, and Runs on demand (Run heartbeat, assignments, comments)
// while WakeOnDemand.
type HeartbeatPolicy struct {
	Enabled      bool
	IntervalSec  int
	WakeOnDemand bool
}

// DefaultHeartbeatPolicy is the policy of a new Agent: no timer, every 300
// seconds once switched on, woken on demand.
func DefaultHeartbeatPolicy() HeartbeatPolicy {
	return HeartbeatPolicy{IntervalSec: 300, WakeOnDemand: true}
}

// HeartbeatPatch changes a HeartbeatPolicy: a nil field stays as it is.
type HeartbeatPatch struct {
	Enabled      *bool
	IntervalSec  *int
	WakeOnDemand *bool
}

func (p HeartbeatPatch) applied(to HeartbeatPolicy) (HeartbeatPolicy, error) {
	if p.Enabled != nil {
		to.Enabled = *p.Enabled
	}
	if p.IntervalSec != nil {
		to.IntervalSec = *p.IntervalSec
	}
	if p.WakeOnDemand != nil {
		to.WakeOnDemand = *p.WakeOnDemand
	}
	if to.IntervalSec < MinHeartbeatInterval || to.IntervalSec > MaxHeartbeatInterval {
		return HeartbeatPolicy{}, invalid("heartbeat.interval_sec", "must be %d to %d seconds", MinHeartbeatInterval, MaxHeartbeatInterval)
	}
	return to, nil
}

// Job is what an Agent does in the Org chart (Paperclip's agent role).
type Job string

// Jobs are the glossary's Jobs in Paperclip's order, with their labels.
var Jobs = []Job{"ceo", "cto", "cmo", "cfo", "security", "engineer", "designer", "pm", "qa", "devops", "researcher", "general"}

var jobLabels = map[Job]string{
	"ceo": "CEO", "cto": "CTO", "cmo": "CMO", "cfo": "CFO", "security": "Security", "engineer": "Engineer",
	"designer": "Designer", "pm": "PM", "qa": "QA", "devops": "DevOps", "researcher": "Researcher", "general": "General",
}

// DefaultJob is the Job of an Agent hired without one.
const DefaultJob Job = "general"

// JobLabel is how the Job reads, e.g. "DevOps".
func JobLabel(j Job) string { return jobLabels[j] }

// ParseJob reads a Job; empty is DefaultJob.
func ParseJob(s string) (Job, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultJob, nil
	}
	if slices.Contains(Jobs, Job(s)) {
		return Job(s), nil
	}
	return "", invalid("job", "must be one of %s", joined(Jobs))
}

// Icon is an Agent icon, drawn with the Lucide icon of its name.
type Icon string

// Icons are Paperclip's AGENT_ICON_NAMES.
var Icons = []Icon{
	"bot", "cpu", "brain", "zap", "rocket", "code", "terminal", "shield", "eye", "search", "wrench", "hammer",
	"lightbulb", "sparkles", "star", "heart", "flame", "bug", "cog", "database", "globe", "lock", "mail",
	"message-square", "file-code", "git-branch", "package", "puzzle", "target", "wand", "atom", "circuit-board",
	"radar", "swords", "telescope", "microscope", "crown", "gem", "hexagon", "pentagon", "fingerprint",
}

// ParseIcon reads an Agent icon; empty is none.
func ParseIcon(s string) (Icon, error) {
	s = strings.TrimSpace(s)
	if s == "" || slices.Contains(Icons, Icon(s)) {
		return Icon(s), nil
	}
	return "", invalid("icon", "is not an agent icon")
}

func joined[T ~string](vs []T) string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return strings.Join(out, ", ")
}

// Status is an Agent status.
type Status string

const (
	PendingApproval Status = "pending_approval"
	Idle            Status = "idle"
	Running         Status = "running"
	Error           Status = "error"
	Paused          Status = "paused"
	Terminated      Status = "terminated"
)

// Agent is an AI worker hired into one Guild by its Hirer. ManagerID is 0
// for the top of the Org chart; HireApprovalID is its hire_agent Approval.
// LastHeartbeatAt is when its last timer Run was claimed.
type Agent struct {
	ID              uint64
	GuildID         uint64
	HirerID         uint64
	Name            string
	Job             Job
	Title           string
	Icon            Icon
	Capabilities    string
	ManagerID       uint64
	Status          Status
	HireApprovalID  uint64
	Heartbeat       HeartbeatPolicy
	LastHeartbeatAt *time.Time
	PausedAt        *time.Time
	TerminatedAt    *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Profile is what a person types about an Agent.
type Profile struct {
	Name         string
	Job          string
	Title        string
	Icon         string
	Capabilities string
}

func (p Profile) validated() (name string, job Job, title string, icon Icon, caps string, err error) {
	name = strings.TrimSpace(p.Name)
	if n := utf8.RuneCountInString(name); n == 0 || n > MaxName {
		return "", "", "", "", "", invalid("name", "must be 1 to %d characters", MaxName)
	}
	if job, err = ParseJob(p.Job); err != nil {
		return "", "", "", "", "", err
	}
	title = strings.TrimSpace(p.Title)
	if utf8.RuneCountInString(title) > MaxTitle {
		return "", "", "", "", "", invalid("title", "must be at most %d characters", MaxTitle)
	}
	if icon, err = ParseIcon(p.Icon); err != nil {
		return "", "", "", "", "", err
	}
	caps = strings.TrimSpace(p.Capabilities)
	if utf8.RuneCountInString(caps) > MaxCapabilities {
		return "", "", "", "", "", invalid("capabilities", "must be at most %d characters", MaxCapabilities)
	}
	return name, job, title, icon, caps, nil
}

// Hire makes a new Agent of the Guild, pending its hire_agent Approval.
// The Manager is checked against the Guild's Agents with CheckManager.
func Hire(guildID, hirerID uint64, p Profile, managerID uint64, at time.Time) (Agent, error) {
	name, job, title, icon, caps, err := p.validated()
	if err != nil {
		return Agent{}, err
	}
	return Agent{
		GuildID: guildID, HirerID: hirerID, Name: name, Job: job, Title: title, Icon: icon, Capabilities: caps,
		ManagerID: managerID, Status: PendingApproval, Heartbeat: DefaultHeartbeatPolicy(), CreatedAt: at, UpdatedAt: at,
	}, nil
}

// ErrNameTaken is a name another Agent of the Guild that is not terminated
// already has, in any case.
var ErrNameTaken error = &FieldError{Field: "name", Message: "another agent of this guild has this name"}

// ErrManagerCycle is a Manager that is the Agent itself or one of its
// reports.
var ErrManagerCycle error = &FieldError{Field: "reports_to", Message: "an agent cannot report to itself or to one of its reports"}

// CheckManager checks managerID as the Manager of the Agent id (0 for one
// not hired yet) among the Guild's Agents: one of them, not terminated,
// not the Agent and not in its reports, found by walking the Manager's
// Chain of command up at most MaxChain levels.
func CheckManager(agents []Agent, id, managerID uint64) error {
	if managerID == 0 {
		return nil
	}
	byID := make(map[uint64]Agent, len(agents))
	for _, a := range agents {
		byID[a.ID] = a
	}
	m, ok := byID[managerID]
	if !ok {
		return invalid("reports_to", "is not an agent of this guild")
	}
	if m.Status == Terminated {
		return invalid("reports_to", "is terminated")
	}
	if id == 0 {
		return nil
	}
	for at, i := m, 0; i < MaxChain; i++ {
		if at.ID == id {
			return ErrManagerCycle
		}
		next, ok := byID[at.ManagerID]
		if at.ManagerID == 0 || !ok {
			return nil
		}
		at = next
	}
	return nil
}

// Approve follows an approved hire_agent Approval: pending_approval
// becomes idle. An Agent already past it stays as it is (a Decision made
// again); a terminated one cannot be approved.
func (a *Agent) Approve(at time.Time) (changed bool, err error) {
	switch a.Status {
	case PendingApproval:
		a.Status, a.UpdatedAt = Idle, at
		return true, nil
	case Idle, Running, Error, Paused:
		return false, nil
	}
	return false, &StatusError{Status: a.Status, Action: "approved"}
}

// Reject follows a rejected hire_agent Approval: pending_approval becomes
// terminated. A terminated Agent stays as it is; a hired one cannot be
// rejected.
func (a *Agent) Reject(at time.Time) (changed bool, err error) {
	switch a.Status {
	case PendingApproval:
		a.Status, a.TerminatedAt, a.UpdatedAt = Terminated, &at, at
		return true, nil
	case Terminated:
		return false, nil
	}
	return false, &StatusError{Status: a.Status, Action: "rejected"}
}

// Patch changes an Agent's Profile, Manager and Heartbeat policy: a nil
// field stays as it is, a ManagerID of 0 reports to no one.
type Patch struct {
	Name, Job, Title, Icon, Capabilities *string
	ManagerID                            *uint64
	Heartbeat                            *HeartbeatPatch
}

// HeartbeatOnly reports whether the Patch changes the Heartbeat policy
// and nothing else.
func (p Patch) HeartbeatOnly() bool {
	return p.Heartbeat != nil && p.Name == nil && p.Job == nil && p.Title == nil && p.Icon == nil && p.Capabilities == nil && p.ManagerID == nil
}

// Change is one field's change, as the Activity records it.
type Change struct {
	From, To any
}

// Edit applies the Patch to an idle, error or paused Agent, or a Patch of
// the Heartbeat policy alone to a running one, and answers what
// changed, keyed by the API's field names (reports_to for the Manager,
// by id; heartbeat.enabled, heartbeat.interval_sec and
// heartbeat.wake_on_demand for the policy). The new Manager is checked
// with CheckManager first.
func (a *Agent) Edit(p Patch, at time.Time) (map[string]Change, error) {
	if a.Status == Running && !p.HeartbeatOnly() || a.Status != Running && a.Status != Idle && a.Status != Error && a.Status != Paused {
		return nil, &StatusError{Status: a.Status, Action: "edited"}
	}
	in := Profile{Name: a.Name, Job: string(a.Job), Title: a.Title, Icon: string(a.Icon), Capabilities: a.Capabilities}
	for _, f := range []struct{ to, from *string }{{&in.Name, p.Name}, {&in.Job, p.Job}, {&in.Title, p.Title}, {&in.Icon, p.Icon}, {&in.Capabilities, p.Capabilities}} {
		if f.from != nil {
			*f.to = *f.from
		}
	}
	name, job, title, icon, caps, err := in.validated()
	if err != nil {
		return nil, err
	}
	manager := a.ManagerID
	if p.ManagerID != nil {
		manager = *p.ManagerID
	}
	heartbeat := a.Heartbeat
	if p.Heartbeat != nil {
		if heartbeat, err = p.Heartbeat.applied(a.Heartbeat); err != nil {
			return nil, err
		}
	}
	changes := map[string]Change{}
	note := func(field string, from, to any) {
		if from != to {
			changes[field] = Change{From: from, To: to}
		}
	}
	note("name", a.Name, name)
	note("job", string(a.Job), string(job))
	note("title", a.Title, title)
	note("icon", string(a.Icon), string(icon))
	note("capabilities", a.Capabilities, caps)
	note("reports_to", a.ManagerID, manager)
	note("heartbeat.enabled", a.Heartbeat.Enabled, heartbeat.Enabled)
	note("heartbeat.interval_sec", a.Heartbeat.IntervalSec, heartbeat.IntervalSec)
	note("heartbeat.wake_on_demand", a.Heartbeat.WakeOnDemand, heartbeat.WakeOnDemand)
	if len(changes) > 0 {
		a.Name, a.Job, a.Title, a.Icon, a.Capabilities, a.ManagerID, a.UpdatedAt = name, job, title, icon, caps, manager, at
		a.Heartbeat = heartbeat
	}
	return changes, nil
}

// Pause makes an idle, running or error Agent paused; its Runs are
// cancelled with it.
func (a *Agent) Pause(at time.Time) error {
	if a.Status != Idle && a.Status != Running && a.Status != Error {
		return &StatusError{Status: a.Status, Action: "paused"}
	}
	a.Status, a.PausedAt, a.UpdatedAt = Paused, &at, at
	return nil
}

// Resume makes a paused Agent idle again.
func (a *Agent) Resume(at time.Time) error {
	if a.Status != Paused {
		return &StatusError{Status: a.Status, Action: "resumed"}
	}
	a.Status, a.PausedAt, a.UpdatedAt = Idle, nil, at
	return nil
}

// Terminate ends an Agent in any status but terminated, for good.
func (a *Agent) Terminate(at time.Time) error {
	if a.Status == Terminated {
		return &StatusError{Status: a.Status, Action: "terminated"}
	}
	a.Status, a.TerminatedAt, a.UpdatedAt = Terminated, &at, at
	return nil
}

// StartRunning makes an idle or error Agent running, as a Run of it is
// claimed.
func (a *Agent) StartRunning(at time.Time) error {
	if a.Status != Idle && a.Status != Error {
		return &StatusError{Status: a.Status, Action: "running"}
	}
	a.Status, a.UpdatedAt = Running, at
	return nil
}

// HeartbeatDue reports whether the timer owes the Agent a Heartbeat at
// now: its policy has the timer on, it is idle, running or in error, and
// a whole interval has passed since its last timer Run was claimed (or
// since it was hired, before the first).
func (a Agent) HeartbeatDue(now time.Time) bool {
	if !a.Heartbeat.Enabled || (a.Status != Idle && a.Status != Running && a.Status != Error) {
		return false
	}
	last := a.CreatedAt
	if a.LastHeartbeatAt != nil {
		last = *a.LastHeartbeatAt
	}
	return !now.Before(last.Add(time.Duration(a.Heartbeat.IntervalSec) * time.Second))
}

// RunEnded follows the end of the running Run of a running Agent: idle
// after it succeeded or was cancelled, error after it failed or was lost.
// An Agent no longer running (paused or terminated meanwhile) stays as it
// is.
func (a *Agent) RunEnded(s RunStatus, at time.Time) (changed bool) {
	if a.Status != Running {
		return false
	}
	a.Status, a.UpdatedAt = Idle, at
	if s == RunFailed || s == RunLost {
		a.Status = Error
	}
	return true
}

// Rolable reports whether the Agent may be given or lose Roles: only an
// idle, error or paused one, so a pending hire's Approval shows what it
// gets.
func (a Agent) Rolable() error {
	if a.Status != Idle && a.Status != Error && a.Status != Paused {
		return &StatusError{Status: a.Status, Action: "given roles"}
	}
	return nil
}

// Node is an Agent in the Org chart with its direct reports.
type Node struct {
	Agent   Agent
	Reports []Node
}

// Org builds the Org chart of the Guild's Agents that are not terminated:
// roots are those without a Manager or whose Manager is gone, each level
// ordered by name.
func Org(agents []Agent) []Node {
	live := map[uint64]bool{}
	for _, a := range agents {
		if a.Status != Terminated {
			live[a.ID] = true
		}
	}
	children := map[uint64][]Agent{}
	for _, a := range agents {
		if a.Status == Terminated {
			continue
		}
		parent := a.ManagerID
		if !live[parent] {
			parent = 0
		}
		children[parent] = append(children[parent], a)
	}
	seen := map[uint64]bool{}
	var build func(parent uint64) []Node
	build = func(parent uint64) []Node {
		kids := children[parent]
		slices.SortFunc(kids, func(x, y Agent) int {
			if c := strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)); c != 0 {
				return c
			}
			return int(x.ID) - int(y.ID)
		})
		out := make([]Node, 0, len(kids))
		for _, k := range kids {
			if seen[k.ID] {
				continue
			}
			seen[k.ID] = true
			out = append(out, Node{Agent: k, Reports: build(k.ID)})
		}
		return out
	}
	return build(0)
}

// AgentHired is published when a Member has hired an Agent.
type AgentHired struct {
	Agent   Agent
	ActorID uint64
}

// AgentUpdated is published when a Member has edited an Agent.
type AgentUpdated struct {
	Agent   Agent
	ActorID uint64
	Changes map[string]Change
}

// AgentPaused is published when a Member has paused an Agent.
type AgentPaused struct {
	Agent   Agent
	ActorID uint64
}

// AgentResumed is published when a Member has resumed an Agent.
type AgentResumed struct {
	Agent   Agent
	ActorID uint64
}

// AgentRoleAdded is published when a Member has given an Agent a Role.
type AgentRoleAdded struct {
	Agent   Agent
	ActorID uint64
	Role    string
}

// AgentRoleRemoved is published when a Member has taken a Role from an
// Agent.
type AgentRoleRemoved struct {
	Agent   Agent
	ActorID uint64
	Role    string
}

// AgentTerminated is published when an Agent became terminated: by a
// person, by its rejected hire_agent Approval, or by its Hirer leaving.
type AgentTerminated struct {
	Agent   Agent
	ActorID uint64
}
