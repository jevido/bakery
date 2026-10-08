// Package work is what the router and other contexts may use from the
// work context: its routes (Goals, Issues, Comments, Issue documents, the
// Activity, the Inbox and Approvals), and for the agents context
// RequestApproval, CancelApproval, OnApprovalDecided, RecordActivity,
// OnAgentNames, OnAgentAssignees, UnassignAgent, IssueForRun,
// OpenIssuesOfAgent, CommentsForRun, OnIssueAssigned and OnIssueCommented.
// Nothing else in contexts/work is for outside use.
package work

import (
	"context"
	"sync"

	"github.com/goravel/framework/contracts/route"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/guilds"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/projects"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
	workhttp "github.com/jevido/bakery/services/api/contexts/work/http"
	"github.com/jevido/bakery/services/api/contexts/work/infra"
)

var (
	once    sync.Once
	service *app.Service
)

func svc() *app.Service {
	once.Do(func() {
		service = app.NewService(infra.Goals{}, infra.Issues{}, infra.Comments{}, infra.Documents{}, guildsOfWork{}, projectsOfWork{}, infra.Activity{}, infra.Inbox{}, infra.Approvals{})
		service.Logf = facades.Log().Errorf
		service.Decided = approvalDecided
		service.Agents = assigneeAgents
		service.Assigned = issueAssigned
		service.Commented = issueCommented
		guilds.OnGuildDeleting("goals", func(ctx context.Context, guildID uint64) (bool, error) {
			gs, err := service.Goals(ctx, guildID)
			return len(gs) > 0, err
		})
		guilds.OnGuildDeleting("issues", service.HasIssues)
		projects.OnProjectDeleted(service.ForgetProject)
	})
	return service
}

// HireAgentRequest is the Agent a hire_agent Approval asks the Board to
// Hire, as the agents context describes it. ManagerID is 0 when it reports
// to no one; Roles are the names of the Roles it would get.
type HireAgentRequest struct {
	AgentID      uint64
	Name         string
	Job          string
	Title        string
	Icon         string
	Capabilities string
	ManagerID    uint64
	ManagerName  string
	Roles        []string
}

// RequestApproval asks the Guild's Board for a hire_agent Approval with the
// Hirer as its Requester, and answers its id. The caller has checked that
// the Hirer may hire.
func RequestApproval(ctx context.Context, guildID, hirerID uint64, r HireAgentRequest) (uint64, error) {
	a, err := svc().RequestHireApproval(ctx, guildID, hirerID, domain.HireAgentPayload{
		AgentID: r.AgentID, Name: r.Name, Job: r.Job, Title: r.Title, Icon: r.Icon,
		Capabilities: r.Capabilities, ManagerID: r.ManagerID, ManagerName: r.ManagerName, Roles: r.Roles,
	})
	return a.ID, err
}

// CancelApproval cancels the Guild's hire_agent Approval while it waits for
// a Decision, because the Member terminated its Agent; the Activity names
// them as its Actor. A cancelled one stays as it is.
func CancelApproval(ctx context.Context, guildID, actorID, approvalID uint64) error {
	_, err := svc().CancelApproval(ctx, guildID, actorID, approvalID)
	return err
}

// ApprovalDecided is a Decision on an Approval, once it is stored: its
// type, whether it was approved (else rejected), who decided, and for a
// hire_agent the Agent it is about.
type ApprovalDecided struct {
	GuildID    uint64
	ApprovalID uint64
	Type       string
	Approved   bool
	AgentID    uint64
	DeciderID  uint64
}

var (
	decidedMu sync.RWMutex
	onDecided = map[string]func(ctx context.Context, d ApprovalDecided) error{}
	agentsMu  sync.RWMutex
	onNames   func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error)
	onAssign  func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]AssigneeAgent, error)
)

// AssigneeAgent is an Agent as an Issue's Assignee shows it: its name, its
// Agent icon and whether it is terminated, which keeps it from being
// assigned.
type AssigneeAgent struct {
	Name       string
	Icon       string
	Terminated bool
}

// OnAgentAssignees registers f to name the Guild's Agents among ids,
// terminated ones too, so an Issue can be assigned to one and show it.
// Until it is registered, no Agent can be an Assignee.
func OnAgentAssignees(f func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]AssigneeAgent, error)) {
	agentsMu.Lock()
	defer agentsMu.Unlock()
	onAssign = f
}

func assigneeAgents(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]app.AssigneeAgent, error) {
	agentsMu.RLock()
	f := onAssign
	agentsMu.RUnlock()
	if f == nil {
		return nil, nil
	}
	as, err := f(ctx, guildID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]app.AssigneeAgent, len(as))
	for id, a := range as {
		out[id] = app.AssigneeAgent(a)
	}
	return out, nil
}

// UnassignAgent takes the terminated Agent off the Guild's Issues that are
// not done or cancelled; each change is recorded with actorID, who
// terminated it, as its Actor.
func UnassignAgent(ctx context.Context, guildID, agentID, actorID uint64) error {
	return svc().UnassignAgent(ctx, guildID, agentID, actorID)
}

// IssueBrief is an Issue as the agents context needs it for a Run: its
// Issue identifier, title, description and status, its Project (0 for
// none) and its Agent assignee (0 for none).
type IssueBrief struct {
	ID              uint64
	ProjectID       uint64
	Identifier      string
	Title           string
	Description     string
	Status          string
	AgentAssigneeID uint64
}

// IssueForRun tells the Guild's Issue; found is false when it is
// another Guild's or there is none. It does not check who may view it.
func IssueForRun(ctx context.Context, guildID, issueID uint64) (IssueBrief, bool, error) {
	i, prefix, found, err := svc().IssueOfGuild(ctx, guildID, issueID)
	if err != nil || !found {
		return IssueBrief{}, false, err
	}
	return IssueBrief{
		ID: i.ID, ProjectID: i.ProjectID, Identifier: domain.Identifier(prefix, i.Number), Title: i.Title,
		Description: i.Description, Status: string(i.Status), AgentAssigneeID: i.AssigneeAgentID,
	}, true, nil
}

// OpenIssuesOfAgent lists the Guild's Issues the Agent is the assignee of
// that are todo, in progress or in review, by Issue identifier, whatever
// Project they are in.
func OpenIssuesOfAgent(ctx context.Context, guildID, agentID uint64) ([]IssueBrief, error) {
	is, prefix, err := svc().AgentWork(ctx, guildID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]IssueBrief, len(is))
	for n, i := range is {
		out[n] = IssueBrief{
			ID: i.ID, ProjectID: i.ProjectID, Identifier: domain.Identifier(prefix, i.Number), Title: i.Title,
			Description: i.Description, Status: string(i.Status), AgentAssigneeID: i.AssigneeAgentID,
		}
	}
	return out, nil
}

// RunComment is a Comment as a Run's prompt quotes it, with its author's
// name: a Member's or an Agent's.
type RunComment struct {
	ID         uint64
	AuthorName string
	Body       string
}

// CommentsForRun tells the Guild's Comments with these ids, in that order;
// deleted ones and ids that are not the Guild's are left out.
func CommentsForRun(ctx context.Context, guildID uint64, ids []uint64) ([]RunComment, error) {
	cs, err := svc().CommentsOfGuild(ctx, guildID, ids)
	if err != nil || len(cs) == 0 {
		return nil, err
	}
	var members, agents []uint64
	for _, c := range cs {
		if c.Author.MemberID != 0 {
			members = append(members, c.Author.MemberID)
		}
		if c.Author.AgentID != 0 {
			agents = append(agents, c.Author.AgentID)
		}
	}
	ms, err := memberNames(ctx, members)
	if err != nil {
		return nil, err
	}
	names := make(map[uint64]string, len(ms))
	for _, m := range ms {
		names[m.ID] = m.Name
	}
	as, err := svc().AssigneeAgents(ctx, guildID, agents)
	if err != nil {
		return nil, err
	}
	out := make([]RunComment, len(cs))
	for n, c := range cs {
		out[n] = RunComment{ID: c.ID, AuthorName: names[c.Author.MemberID], Body: c.Body}
		if c.Author.AgentID != 0 {
			out[n].AuthorName = as[c.Author.AgentID].Name
		}
	}
	return out, nil
}

// IssueAssigned is an Issue, once stored, that its Agent assignee now has
// to work on: assigned to it while open and out of the backlog, or moved
// out of the backlog while it is the Assignee. ActorID is the person who
// changed it.
type IssueAssigned struct {
	GuildID uint64
	IssueID uint64
	AgentID uint64
	ActorID uint64
}

// IssueCommented is a Comment, once stored, on an Issue an Agent is the
// Assignee of and that is not done or cancelled, written by anyone but
// that Agent. ActorID is its author when a Member wrote it, else 0.
type IssueCommented struct {
	GuildID   uint64
	IssueID   uint64
	AgentID   uint64
	CommentID uint64
	ActorID   uint64
}

var (
	hooksMu     sync.RWMutex
	onAssigned  func(ctx context.Context, e IssueAssigned) error
	onCommented func(ctx context.Context, e IssueCommented) error
)

// OnIssueAssigned registers f to hear every IssueAssigned. Its error is
// logged; the change it follows still succeeds.
func OnIssueAssigned(f func(ctx context.Context, e IssueAssigned) error) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	onAssigned = f
}

// OnIssueCommented registers f to hear every IssueCommented. Its error is
// logged; the Comment is still written.
func OnIssueCommented(f func(ctx context.Context, e IssueCommented) error) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	onCommented = f
}

func issueAssigned(ctx context.Context, i domain.Issue, actorID uint64) error {
	hooksMu.RLock()
	f := onAssigned
	hooksMu.RUnlock()
	if f == nil {
		return nil
	}
	return f(ctx, IssueAssigned{GuildID: i.GuildID, IssueID: i.ID, AgentID: i.AssigneeAgentID, ActorID: actorID})
}

func issueCommented(ctx context.Context, i domain.Issue, c domain.Comment) error {
	hooksMu.RLock()
	f := onCommented
	hooksMu.RUnlock()
	if f == nil {
		return nil
	}
	return f(ctx, IssueCommented{GuildID: i.GuildID, IssueID: i.ID, AgentID: i.AssigneeAgentID, CommentID: c.ID, ActorID: c.Author.MemberID})
}

// OnApprovalDecided registers f to hear every approve or reject of an
// Approval of the type. Making the same Decision again calls f again, so
// a failed f is healed by deciding again; its error answers the Decision
// 500, after the Decision is stored.
func OnApprovalDecided(typ string, f func(ctx context.Context, d ApprovalDecided) error) {
	decidedMu.Lock()
	defer decidedMu.Unlock()
	onDecided[typ] = f
}

func approvalDecided(ctx context.Context, a domain.Approval) error {
	decidedMu.RLock()
	f := onDecided[string(a.Type)]
	decidedMu.RUnlock()
	if f == nil {
		return nil
	}
	d := ApprovalDecided{GuildID: a.GuildID, ApprovalID: a.ID, Type: string(a.Type), Approved: a.Status == domain.StatusApproved, DeciderID: a.DeciderID}
	if h, ok := a.Payload.(domain.HireAgentPayload); ok {
		d.AgentID = h.AgentID
	}
	return f(ctx, d)
}

// AgentActivity is an Activity event about an Agent: its Actor (a Member,
// or with ActorAgentID an Agent), one of
// the glossary's agent.* and run.* Actions, the Agent's name (kept, so the event
// still reads after a rename) and the details its Action carries.
type AgentActivity struct {
	GuildID      uint64
	ActorID      uint64
	ActorAgentID uint64
	AgentID      uint64
	Action       string
	AgentName    string
	Details      map[string]any
}

// RecordActivity adds the event to the Guild's Activity. Anything but an
// agent.* or run.* Action is refused.
func RecordActivity(ctx context.Context, e AgentActivity) error {
	return svc().RecordAgentActivity(ctx, domain.AgentEvent{
		Happened: domain.Happened{Actor: domain.Actor{MemberID: e.ActorID, AgentID: e.ActorAgentID}}, GuildID: e.GuildID, AgentID: e.AgentID,
		AgentName: e.AgentName, Action: e.Action, Details: e.Details,
	})
}

// OnAgentNames registers f to name the Guild's Agents among ids that still
// exist, so the Activity can tell a terminated or deleted one. Until it is
// registered, every Agent counts as existing.
func OnAgentNames(f func(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error)) {
	agentsMu.Lock()
	defer agentsMu.Unlock()
	onNames = f
}

func agentNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	agentsMu.RLock()
	f := onNames
	agentsMu.RUnlock()
	if f == nil {
		return nil, nil
	}
	return f(ctx, guildID, ids)
}

// guildsOfWork is guilds' answer about Members and Issue prefixes, in
// work's terms.
type guildsOfWork struct{}

func (guildsOfWork) IsMember(ctx context.Context, guildID, memberID uint64) (bool, error) {
	return guilds.IsMember(ctx, guildID, memberID)
}

func (guildsOfWork) IssuePrefix(ctx context.Context, guildID uint64) (string, error) {
	return guilds.IssuePrefix(ctx, guildID)
}

// projectsOfWork is projects' answer about Project names.
type projectsOfWork struct{}

func (projectsOfWork) ProjectNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	return projects.ProjectNames(ctx, guildID, ids)
}

func memberNames(ctx context.Context, ids []uint64) ([]workhttp.Member, error) {
	ms, err := identity.Members(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]workhttp.Member, len(ms))
	for i, m := range ms {
		out[i] = workhttp.Member{ID: m.ID, Name: m.Name}
	}
	return out, nil
}

// goalInGuild answers 404 for a route whose {id} Goal is another Guild's.
var goalInGuild = guilds.Owns("goal", func(ctx context.Context, id, guildID uint64) (bool, error) {
	return svc().GoalInGuild(ctx, id, guildID)
})

// approvalInGuild answers 404 for a route whose {id} Approval is another
// Guild's.
var approvalInGuild = guilds.Owns("approval", func(ctx context.Context, id, guildID uint64) (bool, error) {
	return svc().ApprovalInGuild(ctx, id, guildID)
})

// Routes registers the Current guild's Goals, Issues, Comments, Issue
// documents, Activity, Inbox and Approvals API: reading (and a Member's own Read marks
// and Inbox archives) needs view_resources, changing manage_work, deciding an
// Approval approve, and
// changing a Comment also being its author. An Issue's {id} is its id or its
// Issue identifier, so the service, not guilds.Owns, answers 404 for one
// outside the Current guild or in a Project the request may not view.
// Agents (guilds.AuthAgents) may read all of it but the Inbox, and create
// and change Issues, write and change their own Comments, save Issue
// documents, request Approvals and comment on them; deleting, Goal changes,
// Read marks, Inbox archives, Decisions and restoring Revisions stay a
// person's.
func Routes(r route.Router) {
	c := workhttp.NewController(svc(), guilds.Current, memberNames)
	c.Visible, c.Member, c.Agent, c.AgentNames = guilds.VisibleProjects, guilds.MemberID, guilds.AgentID, agentNames
	view, manage := guilds.Can("view_resources"), guilds.Can("manage_work")
	r.Middleware(guilds.AuthAgents, view).Get("/api/goals", c.ListGoals)
	r.Middleware(guilds.Auth, manage).Post("/api/goals", c.CreateGoal)
	r.Middleware(guilds.AuthAgents, goalInGuild, view).Get("/api/goals/{id}", c.ShowGoal)
	r.Middleware(guilds.Auth, goalInGuild, manage).Group(func(r route.Router) {
		r.Patch("/api/goals/{id}", c.UpdateGoal)
		r.Delete("/api/goals/{id}", c.DeleteGoal)
	})
	r.Middleware(guilds.AuthAgents, view).Group(func(r route.Router) {
		r.Get("/api/issues", c.ListIssues)
		r.Get("/api/issues/{id}", c.ShowIssue)
		r.Get("/api/activity", c.ListActivity)
		r.Get("/api/issues/{id}/activity", c.ListIssueActivity)
		r.Get("/api/issues/{id}/comments", c.ListComments)
		r.Get("/api/issues/{id}/documents", c.ListDocuments)
		r.Get("/api/issues/{id}/documents/{key}", c.ShowDocument)
		r.Get("/api/issues/{id}/documents/{key}/revisions", c.ListRevisions)
		r.Get("/api/approvals", c.ListApprovals)
		r.Get("/api/issues/{id}/approvals", c.ListIssueApprovals)
	})
	r.Middleware(guilds.AuthAgents, manage).Group(func(r route.Router) {
		r.Post("/api/issues", c.CreateIssue)
		r.Patch("/api/issues/{id}", c.UpdateIssue)
		r.Post("/api/issues/{id}/comments", c.WriteComment)
		r.Patch("/api/issues/{id}/comments/{comment}", c.EditComment)
		r.Delete("/api/issues/{id}/comments/{comment}", c.DeleteComment)
		r.Put("/api/issues/{id}/documents/{key}", c.SaveDocument)
		r.Post("/api/approvals", c.RequestApproval)
	})
	r.Middleware(guilds.Auth, manage).Group(func(r route.Router) {
		r.Delete("/api/issues/{id}", c.DeleteIssue)
		r.Delete("/api/issues/{id}/documents/{key}", c.DeleteDocument)
		r.Post("/api/issues/{id}/documents/{key}/revisions/{revision}/restore", c.RestoreRevision)
	})
	r.Middleware(guilds.Auth, view).Group(func(r route.Router) {
		r.Post("/api/issues/{id}/read", c.MarkRead)
		r.Delete("/api/issues/{id}/read", c.MarkUnread)
		r.Post("/api/issues/{id}/inbox-archive", c.ArchiveFromInbox)
		r.Delete("/api/issues/{id}/inbox-archive", c.UnarchiveFromInbox)
		r.Get("/api/sidebar-badges", c.SidebarBadges)
	})
	r.Middleware(guilds.AuthAgents, approvalInGuild, view).Group(func(r route.Router) {
		r.Get("/api/approvals/{id}", c.ShowApproval)
		r.Get("/api/approvals/{id}/issues", c.ListApprovalIssues)
		r.Get("/api/approvals/{id}/comments", c.ListApprovalComments)
	})
	r.Middleware(guilds.AuthAgents, approvalInGuild, manage).Post("/api/approvals/{id}/comments", c.AddApprovalComment)
	r.Middleware(guilds.Auth, approvalInGuild, guilds.Can("approve")).Group(func(r route.Router) {
		r.Post("/api/approvals/{id}/approve", c.ApproveApproval)
		r.Post("/api/approvals/{id}/reject", c.RejectApproval)
		r.Post("/api/approvals/{id}/request-revision", c.RequestApprovalRevision)
	})
	r.Middleware(guilds.Auth, approvalInGuild, manage).Post("/api/approvals/{id}/resubmit", c.ResubmitApproval)
}
