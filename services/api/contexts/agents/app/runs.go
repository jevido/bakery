package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// Runs keeps Runs and their Run events.
type Runs interface {
	CreateRun(ctx context.Context, r domain.Run) (domain.Run, error)
	Run(ctx context.Context, id uint64) (domain.Run, bool, error)
	// Runs lists the Runs in the query, newest first.
	Runs(ctx context.Context, q RunQuery) ([]domain.Run, error)
	// SaveRun stores the Run's changes only while it is still in the
	// status from; moved is false when another request moved it first.
	SaveRun(ctx context.Context, r domain.Run, from domain.RunStatus) (moved bool, err error)
	// RunningRuns answers the running Run of each Agent that has one.
	RunningRuns(ctx context.Context, agentIDs []uint64) (map[uint64]uint64, error)
	RunEvents(ctx context.Context, runID uint64, after int64, limit int) ([]domain.RunEvent, error)
	DeskRuns
}

// RunQuery picks a Guild's Runs; 0 and an empty list match any.
type RunQuery struct {
	GuildID  uint64
	AgentID  uint64
	IssueID  uint64
	Statuses []domain.RunStatus
	Limit    int
}

// IssueBrief is what work tells agents about an Issue: enough to check a
// Run's Agent assignee and to write its prompt.
type IssueBrief struct {
	ID              uint64
	ProjectID       uint64
	Identifier      string
	Title           string
	Description     string
	Status          string
	AgentAssigneeID uint64
}

// Visible keeps the Projects among ids that the person asking may view;
// nil keeps all of them.
type Visible func(ids []uint64) ([]uint64, error)

// ErrRunNotFound is a Run that does not exist in the Guild.
var ErrRunNotFound = errors.New("not found")

// The most Runs and Run events one page answers, and how many without a
// limit.
const (
	MaxRuns          = 200
	DefaultRuns      = 50
	MaxRunEvents     = 1000
	DefaultRunEvents = 500
)

// Prompt is what claude is asked in a Run of the Agent on the Issue: the
// Issue as "{identifier}: {title}", a blank line and its description,
// then one line naming the Agent. Heartbeats will replace it.
func Prompt(a domain.Agent, i IssueBrief) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n\n", i.Identifier, i.Title)
	if d := strings.TrimSpace(i.Description); d != "" {
		b.WriteString(d + "\n\n")
	}
	fmt.Fprintf(&b, "You are %s, the guild's %s", a.Name, domain.JobLabel(a.Job))
	if a.Title != "" {
		fmt.Fprintf(&b, " (%s)", a.Title)
	}
	if a.Capabilities != "" {
		fmt.Fprintf(&b, ". Your capabilities: %s", strings.Join(strings.Fields(a.Capabilities), " "))
	}
	b.WriteString(".")
	return b.String()
}

// issueOf is the Guild's Issue as work tells it, when the person may view
// it.
func (s *Service) issueOf(ctx context.Context, guildID, issueID uint64, visible Visible) (IssueBrief, bool, error) {
	i, ok, err := s.work.IssueForRun(ctx, guildID, issueID)
	if err != nil || !ok || i.ProjectID == 0 || visible == nil {
		return i, ok, err
	}
	ids, err := visible([]uint64{i.ProjectID})
	return i, len(ids) == 1, err
}

// StartRun queues a Run of the Agent on the Issue assigned to it, by a
// person who may manage the Agent.
func (s *Service) StartRun(ctx context.Context, guildID uint64, actor Actor, agentID, issueID uint64, visible Visible) (domain.Run, error) {
	a, err := s.managed(ctx, guildID, actor, agentID)
	if err != nil {
		return domain.Run{}, err
	}
	if err := a.Runnable(); err != nil {
		return domain.Run{}, err
	}
	if issueID == 0 {
		return domain.Run{}, &domain.FieldError{Field: "issue_id", Message: "is required"}
	}
	i, ok, err := s.issueOf(ctx, guildID, issueID, visible)
	if err != nil {
		return domain.Run{}, err
	}
	if !ok {
		return domain.Run{}, &domain.FieldError{Field: "issue_id", Message: "is not an issue of this guild"}
	}
	if i.AgentAssigneeID != a.ID {
		return domain.Run{}, &domain.FieldError{Field: "issue_id", Message: "is not assigned to this agent"}
	}
	r, err := domain.StartRun(a, i.ID, actor.ID, domain.OnDemand, Prompt(a, i), s.now())
	if err != nil {
		return domain.Run{}, err
	}
	if r, err = s.runs.CreateRun(ctx, r); err != nil {
		return domain.Run{}, err
	}
	s.recordRun(ctx, domain.RunStarted{Run: r, Agent: a, ActorID: actor.ID, Identifier: i.Identifier})
	return r, nil
}

// Run is the Guild's Run; ErrRunNotFound when it is another Guild's.
func (s *Service) Run(ctx context.Context, guildID, id uint64) (domain.Run, error) {
	r, ok, err := s.runs.Run(ctx, id)
	if err != nil {
		return domain.Run{}, err
	}
	if !ok || r.GuildID != guildID {
		return domain.Run{}, ErrRunNotFound
	}
	return r, nil
}

// RunInGuild reports whether the Run belongs to the Guild.
func (s *Service) RunInGuild(ctx context.Context, id, guildID uint64) (bool, error) {
	r, ok, err := s.runs.Run(ctx, id)
	return ok && r.GuildID == guildID, err
}

// RunFilter picks the Guild's Runs: 0 and "" match any.
type RunFilter struct {
	AgentID uint64
	IssueID uint64
	Status  string
	Limit   int
}

// Runs lists the Guild's Runs in the filter, newest first.
func (s *Service) Runs(ctx context.Context, guildID uint64, f RunFilter) ([]domain.Run, error) {
	q := RunQuery{GuildID: guildID, AgentID: f.AgentID, IssueID: f.IssueID, Limit: f.Limit}
	if f.Status != "" {
		st, err := domain.ParseRunStatus(f.Status)
		if err != nil {
			return nil, err
		}
		q.Statuses = []domain.RunStatus{st}
	}
	if q.Limit <= 0 {
		q.Limit = DefaultRuns
	}
	q.Limit = min(q.Limit, MaxRuns)
	return s.runs.Runs(ctx, q)
}

// RunEvents lists the Guild's Run's events after the seq, by seq.
func (s *Service) RunEvents(ctx context.Context, guildID, runID uint64, after int64, limit int) ([]domain.RunEvent, error) {
	if _, err := s.Run(ctx, guildID, runID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultRunEvents
	}
	return s.runs.RunEvents(ctx, runID, after, min(limit, MaxRunEvents))
}

// RunningRuns answers the running Run of each Agent that has one.
func (s *Service) RunningRuns(ctx context.Context, agentIDs []uint64) (map[uint64]uint64, error) {
	return s.runs.RunningRuns(ctx, agentIDs)
}

// IssueBriefs tells the Issues of the Guild's Runs that the person may
// view, by id; one deleted or hidden is left out.
func (s *Service) IssueBriefs(ctx context.Context, guildID uint64, rs []domain.Run, visible Visible) (map[uint64]IssueBrief, error) {
	out := map[uint64]IssueBrief{}
	seen := map[uint64]bool{}
	for _, r := range rs {
		if seen[r.IssueID] || r.IssueID == 0 {
			continue
		}
		seen[r.IssueID] = true
		i, ok, err := s.issueOf(ctx, guildID, r.IssueID, visible)
		if err != nil {
			return nil, err
		}
		if ok {
			out[r.IssueID] = i
		}
	}
	return out, nil
}

// CancelRun ends a queued or running Run, by a person who may manage its
// Agent.
func (s *Service) CancelRun(ctx context.Context, guildID uint64, actor Actor, runID uint64) (domain.Run, error) {
	r, err := s.Run(ctx, guildID, runID)
	if err != nil {
		return domain.Run{}, err
	}
	a, err := s.managed(ctx, guildID, actor, r.AgentID)
	if err != nil {
		return domain.Run{}, err
	}
	return s.cancelRun(ctx, a, r, actor.ID)
}

// cancelRun cancels the Run; another request that moved it first leaves
// it as that one did, refused with its new status.
func (s *Service) cancelRun(ctx context.Context, a domain.Agent, r domain.Run, actorID uint64) (domain.Run, error) {
	from := r.Status
	if err := r.Cancel(s.now()); err != nil {
		return domain.Run{}, err
	}
	moved, err := s.runs.SaveRun(ctx, r, from)
	if err != nil {
		return domain.Run{}, err
	}
	if !moved {
		now, err := s.Run(ctx, r.GuildID, r.ID)
		if err != nil {
			return domain.Run{}, err
		}
		return domain.Run{}, &domain.RunStatusError{Status: now.Status, Action: "cancelled"}
	}
	if from == domain.RunRunning {
		if a, err = s.runEnded(ctx, a.ID, r.Status); err != nil {
			return domain.Run{}, err
		}
	}
	s.recordRun(ctx, domain.RunFinished{Run: r, Agent: a, ActorID: actorID})
	return r, nil
}

// cancelRuns cancels the Agent's queued and running Runs, as Pause and
// Terminate do.
func (s *Service) cancelRuns(ctx context.Context, a domain.Agent, actorID uint64) error {
	rs, err := s.runs.Runs(ctx, RunQuery{GuildID: a.GuildID, AgentID: a.ID, Statuses: []domain.RunStatus{domain.RunQueued, domain.RunRunning}})
	if err != nil {
		return err
	}
	for _, r := range rs {
		var se *domain.RunStatusError
		if _, err := s.cancelRun(ctx, a, r, actorID); err != nil && !errors.As(err, &se) {
			return err
		}
	}
	return nil
}

// agentRunning makes the Agent running, as a Run of it is claimed.
func (s *Service) agentRunning(ctx context.Context, agentID uint64) (domain.Agent, error) {
	a, ok, err := s.agents.Agent(ctx, agentID)
	if err != nil || !ok {
		return a, err
	}
	if err := a.StartRunning(s.now()); err != nil {
		return domain.Agent{}, err
	}
	return a, s.agents.SaveAgent(ctx, a)
}

// runEnded moves the Agent to idle or error after its running Run ended
// in the status; one paused or terminated meanwhile stays as it is.
func (s *Service) runEnded(ctx context.Context, agentID uint64, st domain.RunStatus) (domain.Agent, error) {
	a, ok, err := s.agents.Agent(ctx, agentID)
	if err != nil || !ok {
		return a, err
	}
	if a.RunEnded(st, s.now()) {
		return a, s.agents.SaveAgent(ctx, a)
	}
	return a, nil
}

// recordRun adds a RunStarted or RunFinished to work's Activity, about the
// Run's Agent; a failure is logged.
func (s *Service) recordRun(ctx context.Context, e any) {
	var (
		act Activity
		r   domain.Run
	)
	switch e := e.(type) {
	case domain.RunStarted:
		r, act = e.Run, activityOf(e.Agent, e.ActorID, "run.started")
		if r.IssueID != 0 {
			act.Details["issue"] = map[string]any{"id": r.IssueID, "identifier": e.Identifier}
		}
	case domain.RunFinished:
		r, act = e.Run, activityOf(e.Agent, e.ActorID, "run.finished")
		act.Details["status"] = string(r.Status)
		if r.IssueID != 0 {
			issue := map[string]any{"id": r.IssueID}
			if i, ok, err := s.work.IssueForRun(ctx, r.GuildID, r.IssueID); err == nil && ok {
				issue["identifier"] = i.Identifier
			}
			act.Details["issue"] = issue
		}
	default:
		return
	}
	act.Details["run_id"] = r.ID
	act.Details["agent"] = map[string]any{"id": act.AgentID, "name": act.AgentName}
	if err := s.work.RecordActivity(ctx, act); err != nil {
		s.Logf("agents: recording %s of run %d: %v", act.Action, r.ID, err)
	}
}
