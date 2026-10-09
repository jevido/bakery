package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// Desktop is the Desktop app asking with its Desktop key: it acts for its
// Member, the Hirer of the Agents whose Runs it takes, in every Guild.
type Desktop struct {
	ID       uint64
	MemberID uint64
}

// ErrOtherDesktop refuses a report on a Run another Desktop of the same
// Member claimed.
var ErrOtherDesktop = errors.New("another desktop claimed this run")

// ErrAgentBusy refuses a claim on a Run whose Agent already has a running
// Run.
var ErrAgentBusy = errors.New("the agent already has a running run")

// DeskRuns is what Runs keeps for the Desktops taking them.
type DeskRuns interface {
	// DesktopRuns lists, oldest first, the queued Runs of the Agents the
	// Member hired that are not paused or terminated, in every Guild, and
	// the running Runs the Desktop holds.
	DesktopRuns(ctx context.Context, memberID, desktopID uint64) ([]domain.Run, error)
	// AppendRunEvents stores the events and the Run's NextSeq, Lease and
	// session id in one transaction, only while the Run is running with
	// NextSeq still from; moved is false otherwise.
	AppendRunEvents(ctx context.Context, r domain.Run, events []domain.RunEvent, from int64) (moved bool, err error)
	// KeepRunLease stores the Run's Lease only while it is running.
	KeepRunLease(ctx context.Context, r domain.Run) (moved bool, err error)
	// ExpiredRuns lists the running Runs whose Lease ran out before at.
	ExpiredRuns(ctx context.Context, at time.Time) ([]domain.Run, error)
}

// QueuedRun is a Run as the Desktop gets it: with its Guild's name, its
// Agent and its Issue (HasIssue is false for none or a deleted one).
type QueuedRun struct {
	Run       domain.Run
	GuildName string
	Agent     domain.Agent
	Issue     IssueBrief
	HasIssue  bool
	// RunKey is the Run key, filled only in ClaimRun's answer: the one
	// time it exists outside the Desktop.
	RunKey string
	// Workspace is where the Run works, filled only in ClaimRun's answer;
	// nil when its Issue names no Application with a git repository.
	Workspace *Workspace
}

// Workspace is a Run's git Worktree as the Desktop makes it: the Issue's
// Application, its repository and branch to start from, and the Agent
// branch to work on.
type Workspace struct {
	ApplicationID   uint64
	ApplicationName string
	Repository      string
	BaseBranch      string
	Branch          string
}

// workspaceOf is the Workspace of a Run on the Issue; nil for no Issue, no
// Application, or one without a git source. An Application that cannot
// be read is logged and leaves the Run without one: the Run still runs.
func (s *Service) workspaceOf(ctx context.Context, guildID uint64, i IssueBrief) *Workspace {
	if i.ID == 0 || i.ApplicationID == 0 {
		return nil
	}
	r, ok, err := s.repositories.ApplicationRepository(ctx, guildID, i.ApplicationID)
	if err != nil {
		s.Logf("agents: the workspace of issue %d: %v", i.ID, err)
		return nil
	}
	if !ok {
		return nil
	}
	return &Workspace{
		ApplicationID: i.ApplicationID, ApplicationName: r.Name, Repository: r.URL,
		BaseBranch: r.Branch, Branch: i.AgentBranch,
	}
}

// DesktopRuns lists the Runs waiting for the Desktop's Member, oldest
// first, and the running ones this Desktop holds, so a Runner that
// restarted sees what it lost.
func (s *Service) DesktopRuns(ctx context.Context, d Desktop) ([]QueuedRun, error) {
	rs, err := s.runs.DesktopRuns(ctx, d.MemberID, d.ID)
	if err != nil {
		return nil, err
	}
	out := make([]QueuedRun, 0, len(rs))
	names := map[uint64]string{}
	agents := map[uint64]domain.Agent{}
	for _, r := range rs {
		q, err := s.queued(ctx, r, names, agents)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// queued fills in the Run's Guild name, Agent and Issue, reading each
// Guild and Agent once.
func (s *Service) queued(ctx context.Context, r domain.Run, names map[uint64]string, agents map[uint64]domain.Agent) (QueuedRun, error) {
	q := QueuedRun{Run: r}
	name, ok := names[r.GuildID]
	if !ok {
		var err error
		if name, err = s.guilds.GuildName(ctx, r.GuildID); err != nil {
			return q, err
		}
		names[r.GuildID] = name
	}
	q.GuildName = name
	a, ok := agents[r.AgentID]
	if !ok {
		var err error
		if a, ok, err = s.agents.Agent(ctx, r.AgentID); err != nil {
			return q, err
		}
		agents[r.AgentID] = a
	}
	q.Agent = a
	if r.IssueID != 0 {
		i, ok, err := s.work.IssueForRun(ctx, r.GuildID, r.IssueID)
		if err != nil {
			return q, err
		}
		q.Issue, q.HasIssue = i, ok
	}
	return q, nil
}

// desktopRun is the Run when its Agent's Hirer is the Desktop's Member;
// ErrRunNotFound otherwise, so another person's Runs stay out of sight.
func (s *Service) desktopRun(ctx context.Context, d Desktop, runID uint64) (domain.Run, domain.Agent, error) {
	r, ok, err := s.runs.Run(ctx, runID)
	if err != nil {
		return domain.Run{}, domain.Agent{}, err
	}
	if !ok {
		return domain.Run{}, domain.Agent{}, ErrRunNotFound
	}
	a, ok, err := s.agents.Agent(ctx, r.AgentID)
	if err != nil {
		return domain.Run{}, domain.Agent{}, err
	}
	if !ok || a.HirerID != d.MemberID {
		return domain.Run{}, domain.Agent{}, ErrRunNotFound
	}
	return r, a, nil
}

// claimed is the Run this Desktop claimed; ErrOtherDesktop for one another
// Desktop holds or nobody claimed yet.
func (s *Service) claimed(ctx context.Context, d Desktop, runID uint64) (domain.Run, domain.Agent, error) {
	r, a, err := s.desktopRun(ctx, d, runID)
	if err != nil {
		return r, a, err
	}
	if r.DesktopID != d.ID {
		return domain.Run{}, domain.Agent{}, ErrOtherDesktop
	}
	return r, a, nil
}

// ClaimRun makes a queued Run running on the Desktop, its Agent running,
// and writes its prompt then, so the Wakes that joined it are in it. One
// claimed already or final is a *domain.RunStatusError, one whose Agent
// runs another Run ErrAgentBusy, one whose Agent was paused or terminated
// a *domain.StatusError, and one in a stopped scope is cancelled with the
// *BudgetBlock's reason and answered that.
func (s *Service) ClaimRun(ctx context.Context, d Desktop, runID uint64) (QueuedRun, error) {
	for range wakeAttempts {
		r, a, err := s.desktopRun(ctx, d, runID)
		if err != nil {
			return QueuedRun{}, err
		}
		if r.Status != domain.RunQueued {
			return QueuedRun{}, &domain.RunStatusError{Status: r.Status, Action: "claimed"}
		}
		if a.Status != domain.Idle && a.Status != domain.Error {
			if a.Status == domain.Running {
				return QueuedRun{}, ErrAgentBusy
			}
			return QueuedRun{}, &domain.StatusError{Status: a.Status, Action: "run"}
		}
		var (
			ws        *Workspace
			projectID uint64
			prompt    string
		)
		if prompt, ws, projectID, err = s.promptOf(ctx, a, r); err != nil {
			return QueuedRun{}, err
		}
		block, err := s.budgetBlock(ctx, r.GuildID, a.ID, projectID)
		if err != nil {
			return QueuedRun{}, err
		}
		if block != nil {
			var se *domain.RunStatusError
			if _, err := s.cancelRunBecause(ctx, a, r, 0, block.Reason); err != nil && !errors.As(err, &se) {
				return QueuedRun{}, err
			}
			return QueuedRun{}, block
		}
		key := newRunKey()
		if err := r.Claim(d.ID, projectID, secret.Hash(key), s.now()); err != nil {
			return QueuedRun{}, err
		}
		r.Prompt = prompt
		moved, err := s.runs.SaveRun(ctx, r, domain.RunQueued)
		if err != nil {
			if s.busy(ctx, a.ID, r.ID) {
				return QueuedRun{}, ErrAgentBusy
			}
			return QueuedRun{}, err
		}
		if !moved {
			now, _, err := s.runs.Run(ctx, r.ID)
			if err != nil {
				return QueuedRun{}, err
			}
			if now.Status == domain.RunQueued {
				continue // a Wake joined it meanwhile: write the prompt again
			}
			return QueuedRun{}, &domain.RunStatusError{Status: now.Status, Action: "claimed"}
		}
		if a, err = s.agentRunning(ctx, a.ID); err != nil {
			return QueuedRun{}, err
		}
		q, err := s.queued(ctx, r, map[uint64]string{}, map[uint64]domain.Agent{a.ID: a})
		q.RunKey, q.Workspace = key, ws
		return q, err
	}
	return QueuedRun{}, fmt.Errorf("agents: run %d kept being joined while it was claimed", runID)
}

// newRunKey mints a Run key: the prefix and 24 random bytes in hex.
func newRunKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b) // never fails: crypto/rand panics instead
	return domain.RunKeyPrefix + hex.EncodeToString(b)
}

// RunKeyHolder is who a Run key speaks for: the Agent, in its Run's
// Guild, during that Run, and the Member who hired it.
type RunKeyHolder struct {
	AgentID       uint64
	GuildID       uint64
	RunID         uint64
	HirerMemberID uint64
}

// RunKeyHolder is the holder of the Run key with this hash; false when no
// running Run has it or its Agent is paused or terminated, so the key
// stops working the moment its Run ends.
func (s *Service) RunKeyHolder(ctx context.Context, keyHash string) (RunKeyHolder, bool, error) {
	r, ok, err := s.runs.RunningRunByKeyHash(ctx, keyHash)
	if err != nil || !ok {
		return RunKeyHolder{}, false, err
	}
	a, ok, err := s.agents.Agent(ctx, r.AgentID)
	if err != nil || !ok || a.Status == domain.Paused || a.Status == domain.Terminated {
		return RunKeyHolder{}, false, err
	}
	return RunKeyHolder{AgentID: a.ID, GuildID: r.GuildID, RunID: r.ID, HirerMemberID: a.HirerID}, true, nil
}

// busy reports whether another Run of the Agent is running, which is what
// a failed claim ran into when the database's one-running rule refused it.
func (s *Service) busy(ctx context.Context, agentID, runID uint64) bool {
	running, err := s.runs.RunningRuns(ctx, []uint64{agentID})
	return err == nil && running[agentID] != 0 && running[agentID] != runID
}

// AppendRunEvents keeps the Run events the claiming Desktop reports, the
// ones it has already ignored, and renews the Lease. A gap is a
// *domain.SeqError, a final Run a *domain.RunStatusError.
func (s *Service) AppendRunEvents(ctx context.Context, d Desktop, runID uint64, events []domain.RunEvent) (domain.Run, error) {
	r, _, err := s.claimed(ctx, d, runID)
	if err != nil {
		return domain.Run{}, err
	}
	from := r.NextSeq
	now := s.now()
	keep, err := r.Append(events, now)
	if err != nil {
		return domain.Run{}, err
	}
	for _, e := range keep {
		if id := domain.SessionOf(e); id != "" {
			r.SessionID = id
		}
	}
	_ = r.KeepLease(now)
	moved, err := s.runs.AppendRunEvents(ctx, r, keep, from)
	if err != nil {
		return domain.Run{}, err
	}
	if !moved {
		return domain.Run{}, s.movedMeanwhile(ctx, r.ID, "reported on")
	}
	return r, nil
}

// movedMeanwhile tells why a conditional write found the Run changed: a
// *domain.RunStatusError when it is final, else a *domain.SeqError from
// where it stands now.
func (s *Service) movedMeanwhile(ctx context.Context, runID uint64, action string) error {
	now, _, err := s.runs.Run(ctx, runID)
	if err != nil {
		return err
	}
	if now.Status != domain.RunRunning {
		return &domain.RunStatusError{Status: now.Status, Action: action}
	}
	return &domain.SeqError{Expected: now.NextSeq}
}

// KeepRunLease renews the Lease of the Run the Desktop claimed, while
// claude is quiet.
func (s *Service) KeepRunLease(ctx context.Context, d Desktop, runID uint64) (domain.Run, error) {
	r, _, err := s.claimed(ctx, d, runID)
	if err != nil {
		return domain.Run{}, err
	}
	if err := r.KeepLease(s.now()); err != nil {
		return domain.Run{}, err
	}
	moved, err := s.runs.KeepRunLease(ctx, r)
	if err != nil {
		return domain.Run{}, err
	}
	if !moved {
		return domain.Run{}, s.movedMeanwhile(ctx, r.ID, "kept")
	}
	return r, nil
}

// Finish is how a Run ended on the Desktop.
type Finish struct {
	Status   string
	ExitCode *int
	Error    string
	Usage    domain.Usage
}

// FinishRun ends the Run the Desktop claimed, with its Run usage, moves
// its Agent to idle or error, and evaluates the Budgets it counts toward. A Run cancelled meanwhile is answered
// as it is: the Runner may race a cancel.
func (s *Service) FinishRun(ctx context.Context, d Desktop, runID uint64, f Finish) (domain.Run, error) {
	r, _, err := s.claimed(ctx, d, runID)
	if err != nil {
		return domain.Run{}, err
	}
	if r.Status == domain.RunCancelled {
		return r, nil
	}
	st, err := domain.ParseRunStatus(f.Status)
	if err != nil {
		return domain.Run{}, err
	}
	if err := r.Finish(st, f.Usage, f.ExitCode, f.Error, s.now()); err != nil {
		return domain.Run{}, err
	}
	moved, err := s.runs.SaveRun(ctx, r, domain.RunRunning)
	if err != nil {
		return domain.Run{}, err
	}
	if !moved {
		now, _, err := s.runs.Run(ctx, r.ID)
		if err != nil {
			return domain.Run{}, err
		}
		if now.Status == domain.RunCancelled {
			return now, nil
		}
		return domain.Run{}, &domain.RunStatusError{Status: now.Status, Action: "finished"}
	}
	a, err := s.runEnded(ctx, r.AgentID, r.Status)
	if err != nil {
		return domain.Run{}, err
	}
	s.recordRun(ctx, domain.RunFinished{Run: r, Agent: a, ActorID: d.MemberID})
	s.evaluateRun(ctx, r)
	return r, nil
}

// SweepLostRuns ends each running Run whose Lease ran out as lost, and
// queues it again for when its Desktop is back, up to
// domain.MaxChainRetries times along a chain. Its Agent shows error until
// a Run of it is claimed again. The conditional write
// makes sure two API processes never both queue one again.
func (s *Service) SweepLostRuns(ctx context.Context) error {
	now := s.now()
	rs, err := s.runs.ExpiredRuns(ctx, now)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if err := s.lose(ctx, r, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) lose(ctx context.Context, r domain.Run, now time.Time) error {
	next, err := r.Lose(now)
	if err != nil {
		return nil // moved on since it was read
	}
	moved, err := s.runs.SaveRun(ctx, r, domain.RunRunning)
	if err != nil || !moved {
		return err
	}
	retries, err := s.retries(ctx, r)
	if err != nil {
		return err
	}
	requeued := retries < domain.MaxChainRetries
	if requeued {
		// A twin queued meanwhile on the same Issue takes this one in.
		if _, _, err := s.queue(ctx, next); err != nil {
			return err
		}
	}
	a, ok, err := s.agents.Agent(ctx, r.AgentID)
	if err != nil || !ok {
		return err
	}
	if a.RunEnded(domain.RunLost, now) {
		if err := s.agents.SaveAgent(ctx, a); err != nil {
			return err
		}
	}
	s.recordRun(ctx, domain.RunFinished{Run: r, Agent: a})
	return nil
}

// retries counts the Runs before r along its retry_of_run_id chain.
func (s *Service) retries(ctx context.Context, r domain.Run) (int, error) {
	n := 0
	for id := r.RetryOfRunID; id != 0 && n < domain.MaxChainRetries; n++ {
		prev, ok, err := s.runs.Run(ctx, id)
		if err != nil {
			return 0, err
		}
		if !ok {
			return n + 1, nil
		}
		id = prev.RetryOfRunID
	}
	return n, nil
}

// DesktopRun is the Run of an Agent the Desktop's Member hired;
// ErrRunNotFound for anyone else's.
func (s *Service) DesktopRun(ctx context.Context, d Desktop, runID uint64) (domain.Run, error) {
	r, _, err := s.desktopRun(ctx, d, runID)
	return r, err
}
