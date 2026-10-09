package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// Runs keeps Runs and their Run events.
type Runs interface {
	// CreateRun stores a new Run; ErrQueuedTwin when it is queued and its
	// Agent already has a queued Run on the same Issue (or on none).
	CreateRun(ctx context.Context, r domain.Run) (domain.Run, error)
	// QueuedRunOf is the Agent's queued Run on the Issue (0 for none).
	QueuedRunOf(ctx context.Context, agentID, issueID uint64) (domain.Run, bool, error)
	// JoinRun stores a Join of the Run only while it is queued with the
	// wake count from; moved is false otherwise.
	JoinRun(ctx context.Context, r domain.Run, from int) (moved bool, err error)
	Run(ctx context.Context, id uint64) (domain.Run, bool, error)
	// RunningRunByKeyHash is the running Run whose Run key has this hash.
	RunningRunByKeyHash(ctx context.Context, keyHash string) (domain.Run, bool, error)
	// Runs lists the Runs in the query, newest first.
	Runs(ctx context.Context, q RunQuery) ([]domain.Run, error)
	// SaveRun stores the Run's changes only while it is still in the
	// status from with its wake count; moved is false when another request
	// moved or joined it first.
	SaveRun(ctx context.Context, r domain.Run, from domain.RunStatus) (moved bool, err error)
	// RunningRuns answers the running Run of each Agent that has one.
	RunningRuns(ctx context.Context, agentIDs []uint64) (map[uint64]uint64, error)
	// LiveRuns tells which of the Runs are running; a missing one is not.
	LiveRuns(ctx context.Context, runIDs []uint64) (map[uint64]bool, error)
	RunEvents(ctx context.Context, runID uint64, after int64, limit int) ([]domain.RunEvent, error)
	DeskRuns
	CostRuns
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
	// ApplicationID is the Issue's Application, 0 for none.
	ApplicationID uint64
	// AgentBranch is the branch an Agent pushes for the Issue.
	AgentBranch string
}

// RunComment is a comment on an Issue as a Run's prompt quotes it.
type RunComment struct {
	ID         uint64
	AuthorName string
	Body       string
}

// Visible keeps the Projects among ids that the person asking may view;
// nil keeps all of them.
type Visible func(ids []uint64) ([]uint64, error)

// ErrRunNotFound is a Run that does not exist in the Guild.
var ErrRunNotFound = errors.New("not found")

// ErrQueuedTwin is a new queued Run refused because its Agent already has
// a queued Run on the same Issue: the Wake joins that one instead.
var ErrQueuedTwin = errors.New("the agent already has a queued run on this issue")

// The most Runs and Run events one page answers, and how many without a
// limit.
const (
	MaxRuns          = 200
	DefaultRuns      = 50
	MaxRunEvents     = 1000
	DefaultRunEvents = 500
)

// PromptFor is what claude is asked in a Run of the Agent for the Wake
// reason. A Run on an Issue gets the Issue as "{identifier}: {title}", a
// blank line and its description, then one line naming the Agent, then
// where it works when the Run has a Workspace, then the comments that woke
// it or joined it; an Assignment Run starts with "You were assigned this
// issue.". A Heartbeat Run without an Issue gets "Heartbeat.", the Agent
// line and its open Issues.
func PromptFor(a domain.Agent, reason domain.WakeReason, i IssueBrief, ws *Workspace, comments []RunComment, open []IssueBrief) string {
	var b strings.Builder
	if i.ID == 0 && (reason == domain.HeartbeatInvoked || reason == domain.HeartbeatTimer) {
		b.WriteString("Heartbeat.\n\n")
		agentLine(&b, a)
		if len(open) == 0 {
			b.WriteString("\n\nYou have no open issues.")
			return b.String()
		}
		b.WriteString("\n\nYour open issues:")
		for _, o := range open {
			fmt.Fprintf(&b, "\n- %s [%s]: %s", o.Identifier, o.Status, o.Title)
		}
		return b.String()
	}
	if reason == domain.IssueAssigned {
		b.WriteString("You were assigned this issue.\n\n")
	}
	if i.ID != 0 {
		fmt.Fprintf(&b, "%s: %s\n\n", i.Identifier, i.Title)
		if d := strings.TrimSpace(i.Description); d != "" {
			b.WriteString(d + "\n\n")
		}
	}
	agentLine(&b, a)
	if ws != nil {
		fmt.Fprintf(&b, "\n\nYou work in a git worktree of %s's repository on branch %s, based on %s. "+
			"Commit your changes, push the branch to origin and open the Pull request with bakeryOpenPullRequest; "+
			"a Preview of it will appear on the Issue.", ws.ApplicationName, ws.Branch, ws.BaseBranch)
	}
	if len(comments) > 0 {
		b.WriteString("\n\nNew comments:")
		for _, c := range comments {
			fmt.Fprintf(&b, "\n\n%s wrote:\n%s", c.AuthorName, strings.TrimSpace(c.Body))
		}
	}
	return b.String()
}

// agentLine names the Agent, its job, title and capabilities.
func agentLine(b *strings.Builder, a domain.Agent) {
	fmt.Fprintf(b, "You are %s, the guild's %s", a.Name, domain.JobLabel(a.Job))
	if a.Title != "" {
		fmt.Fprintf(b, " (%s)", a.Title)
	}
	if a.Capabilities != "" {
		fmt.Fprintf(b, ". Your capabilities: %s", strings.Join(strings.Fields(a.Capabilities), " "))
	}
	b.WriteString(".")
}

// promptOf writes the queued Run's prompt from what work tells now: its
// Issue, the comments its Wakes carry, and for a Heartbeat the Agent's
// open Issues; with it the Run's Workspace, nil for none.
func (s *Service) promptOf(ctx context.Context, a domain.Agent, r domain.Run) (string, *Workspace, uint64, error) {
	var (
		i        IssueBrief
		comments []RunComment
		open     []IssueBrief
		err      error
	)
	if r.IssueID != 0 {
		var ok bool
		if i, ok, err = s.work.IssueForRun(ctx, r.GuildID, r.IssueID); err != nil {
			return "", nil, 0, err
		}
		if !ok {
			i = IssueBrief{}
		}
	} else if open, err = s.work.OpenIssuesOfAgent(ctx, r.GuildID, a.ID); err != nil {
		return "", nil, 0, err
	}
	if len(r.WakeContext.CommentIDs) > 0 {
		if comments, err = s.work.CommentsForRun(ctx, r.GuildID, r.WakeContext.CommentIDs); err != nil {
			return "", nil, 0, err
		}
	}
	ws := s.workspaceOf(ctx, r.GuildID, i)
	return PromptFor(a, r.WakeReason, i, ws, comments, open), ws, i.ProjectID, nil
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
// person who may manage the Agent: an on_demand Wake. joined is true when
// it joined the Agent's queued Run on that Issue.
func (s *Service) StartRun(ctx context.Context, guildID uint64, actor Actor, agentID, issueID uint64, visible Visible) (r domain.Run, joined bool, err error) {
	a, err := s.managed(ctx, guildID, actor, agentID)
	if err != nil {
		return domain.Run{}, false, err
	}
	if err := a.Wakeable(domain.OnDemand); err != nil {
		return domain.Run{}, false, err
	}
	if issueID == 0 {
		return domain.Run{}, false, &domain.FieldError{Field: "issue_id", Message: "is required"}
	}
	i, ok, err := s.issueOf(ctx, guildID, issueID, visible)
	if err != nil {
		return domain.Run{}, false, err
	}
	if !ok {
		return domain.Run{}, false, &domain.FieldError{Field: "issue_id", Message: "is not an issue of this guild"}
	}
	if i.AgentAssigneeID != a.ID {
		return domain.Run{}, false, &domain.FieldError{Field: "issue_id", Message: "is not assigned to this agent"}
	}
	return s.wake(ctx, a, WakeInput{Source: domain.OnDemand, Reason: domain.Manual, IssueID: i.ID, ActorID: actor.ID})
}

// RunHeartbeat queues an on_demand Run of the Agent without an Issue, by
// a person who may manage it: Run heartbeat.
func (s *Service) RunHeartbeat(ctx context.Context, guildID uint64, actor Actor, agentID uint64) (r domain.Run, joined bool, err error) {
	a, err := s.managed(ctx, guildID, actor, agentID)
	if err != nil {
		return domain.Run{}, false, err
	}
	return s.wake(ctx, a, WakeInput{Source: domain.OnDemand, Reason: domain.HeartbeatInvoked, ActorID: actor.ID})
}

// IssueAssigned wakes the Agent an open Issue was just assigned to, on
// that Issue, with the person who assigned it as its Actor. A Wake the
// Agent's status or Heartbeat policy refuses is dropped.
func (s *Service) IssueAssigned(ctx context.Context, guildID, issueID, agentID, actorID uint64) error {
	return dropRefused(s.Wake(ctx, guildID, agentID, WakeInput{Source: domain.Assignment, Reason: domain.IssueAssigned, IssueID: issueID, ActorID: actorID}))
}

// IssueCommented wakes the Agent assignee of an Issue a person just
// commented on, on that Issue, bringing the comment. A Wake the Agent's
// status or Heartbeat policy refuses is dropped.
func (s *Service) IssueCommented(ctx context.Context, guildID, issueID, agentID, commentID, actorID uint64) error {
	return dropRefused(s.Wake(ctx, guildID, agentID, WakeInput{Source: domain.Automation, Reason: domain.IssueCommented, IssueID: issueID, ActorID: actorID, CommentID: commentID}))
}

func dropRefused(_ domain.Run, _ bool, err error) error {
	var (
		status *domain.StatusError
		block  *BudgetBlock
	)
	if errors.As(err, &status) || errors.As(err, new(domain.WakeRefused)) || errors.As(err, &block) {
		return nil
	}
	return err
}

// WakeInput is one Wake: what started it and why, its Issue (0 for none),
// the person behind it (0 for none) and the comment it brings (0 for none).
type WakeInput struct {
	Source    domain.InvocationSource
	Reason    domain.WakeReason
	IssueID   uint64
	ActorID   uint64
	CommentID uint64
}

// Wake is the one way a Run of the Guild's Agent starts. It joins the
// Agent's queued Run on the same Issue (or on none) when there is one,
// and otherwise queues a new Run. A Wake the Agent's status or Heartbeat
// policy refuses is a *domain.StatusError or domain.WakeRefused.
func (s *Service) Wake(ctx context.Context, guildID, agentID uint64, w WakeInput) (r domain.Run, joined bool, err error) {
	a, err := s.Agent(ctx, guildID, agentID)
	if err != nil {
		return domain.Run{}, false, err
	}
	return s.wake(ctx, a, w)
}

// wakeAttempts bounds how often a Wake looks again when another request
// queued, joined or claimed the Run it was about to join.
const wakeAttempts = 5

func (s *Service) wake(ctx context.Context, a domain.Agent, w WakeInput) (domain.Run, bool, error) {
	var wc domain.WakeContext
	if w.CommentID != 0 {
		wc.CommentIDs = []uint64{w.CommentID}
	}
	next, err := domain.StartRun(a, w.IssueID, w.ActorID, w.Source, w.Reason, wc, s.now())
	if err != nil {
		return domain.Run{}, false, err
	}
	var issue IssueBrief
	if w.IssueID != 0 {
		var ok bool
		if issue, ok, err = s.work.IssueForRun(ctx, a.GuildID, w.IssueID); err != nil {
			return domain.Run{}, false, err
		}
		if !ok {
			issue = IssueBrief{}
		}
	}
	block, err := s.budgetBlock(ctx, a.GuildID, a.ID, issue.ProjectID)
	if err != nil {
		return domain.Run{}, false, err
	}
	if block != nil {
		return domain.Run{}, false, block
	}
	r, joined, err := s.queue(ctx, next)
	if err != nil {
		return domain.Run{}, false, err
	}
	if !joined && w.Source != domain.Timer {
		s.recordRun(ctx, domain.RunStarted{Run: r, Agent: a, ActorID: w.ActorID, Identifier: issue.Identifier})
	}
	return r, joined, nil
}

// queue stores the new queued Run, or joins the Agent's queued Run on the
// same Issue: the database's one-queued rule settles two requests racing.
func (s *Service) queue(ctx context.Context, next domain.Run) (domain.Run, bool, error) {
	for range wakeAttempts {
		twin, ok, err := s.runs.QueuedRunOf(ctx, next.AgentID, next.IssueID)
		if err != nil {
			return domain.Run{}, false, err
		}
		if ok {
			from := twin.WakeCount
			if err := twin.Join(next.WakeContext, s.now()); err != nil {
				continue
			}
			moved, err := s.runs.JoinRun(ctx, twin, from)
			if err != nil {
				return domain.Run{}, false, err
			}
			if moved {
				return twin, true, nil
			}
			continue
		}
		r, err := s.runs.CreateRun(ctx, next)
		if errors.Is(err, ErrQueuedTwin) {
			continue
		}
		return r, false, err
	}
	return domain.Run{}, false, fmt.Errorf("agents: the queued run of agent %d kept moving", next.AgentID)
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

// LiveRuns tells which of the Runs are running, for work's Checkouts.
func (s *Service) LiveRuns(ctx context.Context, runIDs []uint64) (map[uint64]bool, error) {
	return s.runs.LiveRuns(ctx, runIDs)
}

// SubscriptionLimits tells, for the queued ones among the Runs, when the
// Subscription limit they wait for resets: their Agent's Hirer has every
// signed-in Desktop at its limit. A Run that waits for none is left out.
func (s *Service) SubscriptionLimits(ctx context.Context, rs []domain.Run, agents map[uint64]domain.Agent) (map[uint64]time.Time, error) {
	var hirers []uint64
	for _, r := range rs {
		if a, ok := agents[r.AgentID]; ok && r.Status == domain.RunQueued && a.HirerID != 0 && !slices.Contains(hirers, a.HirerID) {
			hirers = append(hirers, a.HirerID)
		}
	}
	out := map[uint64]time.Time{}
	if len(hirers) == 0 {
		return out, nil
	}
	limits, err := s.hirerLimits(ctx, hirers)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		if t, ok := limits[agents[r.AgentID].HirerID]; ok && r.Status == domain.RunQueued {
			out[r.ID] = t
		}
	}
	return out, nil
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
	return s.cancelRunBecause(ctx, a, r, actorID, "")
}

// cancelRunBecause cancels the Run with the reason as its error ("" for
// none).
func (s *Service) cancelRunBecause(ctx context.Context, a domain.Agent, r domain.Run, actorID uint64, reason string) (domain.Run, error) {
	from := r.Status
	if err := r.CancelBecause(reason, s.now()); err != nil {
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
