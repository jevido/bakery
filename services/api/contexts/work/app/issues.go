package app

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Issues keeps Issues.
type Issues interface {
	// CreateIssue gives the Issue the Guild's next number and stores it, in
	// one transaction.
	CreateIssue(ctx context.Context, i domain.Issue) (domain.Issue, error)
	Issue(ctx context.Context, id uint64) (domain.Issue, bool, error)
	IssueByNumber(ctx context.Context, guildID uint64, number int) (domain.Issue, bool, error)
	// IssuesByID returns the Issues with these ids that exist, in no order.
	IssuesByID(ctx context.Context, ids []uint64) ([]domain.Issue, error)
	SaveIssue(ctx context.Context, i domain.Issue) error
	// SaveIssueBlockedBy stores the Issue and makes blockedBy its Blockers,
	// in one transaction; memberID is who added the new ones.
	SaveIssueBlockedBy(ctx context.Context, i domain.Issue, blockedBy []uint64, memberID uint64) error
	// Blockers returns the ids of each Issue's Blockers.
	Blockers(ctx context.Context, issueIDs []uint64) (map[uint64][]uint64, error)
	// Blocking returns the ids of the Issues each Issue blocks.
	Blocking(ctx context.Context, issueIDs []uint64) (map[uint64][]uint64, error)
	// BlockedFrom returns the ids of every Issue the Issue blocks, directly
	// or through others.
	BlockedFrom(ctx context.Context, issueID uint64) ([]uint64, error)
	// DeleteIssue deletes the Issue; its Sub-issues lose their parent.
	DeleteIssue(ctx context.Context, id uint64) error
	// FindIssues lists the Guild's Issues that q keeps, most recently
	// updated first.
	FindIssues(ctx context.Context, guildID uint64, q IssueQuery) ([]domain.Issue, error)
	// CountIssues counts the Guild's Issues that q keeps, ignoring its
	// Limit and Offset.
	CountIssues(ctx context.Context, guildID uint64, q IssueQuery) (int64, error)
	// InboxStates is where each Issue stands in the Member's Inbox.
	InboxStates(ctx context.Context, memberID uint64, issueIDs []uint64) (map[uint64]InboxState, error)
	// IssueProjects lists the Projects the Guild's Issues are in.
	IssueProjects(ctx context.Context, guildID uint64) ([]uint64, error)
	// LeaveProject takes every Issue, and every Activity event, out of the
	// Project.
	LeaveProject(ctx context.Context, projectID uint64) error
	HasIssues(ctx context.Context, guildID uint64) (bool, error)
	// OpenIssuesOfAgent lists the Guild's Issues assigned to the Agent that
	// are not done or cancelled, whatever Project they are in.
	OpenIssuesOfAgent(ctx context.Context, guildID, agentID uint64) ([]domain.Issue, error)
}

// AssigneeAgent is an Agent as an Issue's Assignee shows it.
type AssigneeAgent struct {
	Name       string
	Icon       string
	Terminated bool
}

// Projects names the Guild's Projects among ids (projects.ProjectNames); an
// id that is not one of them is left out.
type Projects interface {
	ProjectNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error)
}

// Visible keeps the Projects among ids that the person asking may view
// (guilds.VisibleProjects for one request).
type Visible func(ids []uint64) ([]uint64, error)

// MaxIssues is the most Issues one list answers.
const MaxIssues = 200

// IssueFilter is what a list of Issues keeps. Nil fields keep every Issue;
// an id of 0 keeps the Issues without one. Statuses and Priorities keep the
// Issues with any of them.
type IssueFilter struct {
	Statuses   []string
	Priorities []string
	// AssigneeID 0 keeps the Issues with no Assignee at all.
	AssigneeID      *uint64
	AssigneeAgentID *uint64
	ProjectID       *uint64
	GoalID          *uint64
	ParentID        *uint64
	// TouchedBy keeps the Issues the Member is Touched by, UnreadFor the
	// Touched ones that are Unread to them, and InboxFor the Touched ones
	// in their Mine tab: not in their Inbox archive, or Resurfaced since.
	TouchedBy *uint64
	UnreadFor *uint64
	InboxFor  *uint64
	// Search matches the title, the description and the Issue identifier,
	// ignoring case.
	Search string
	// Limit is at most MaxIssues; 0 is MaxIssues.
	Limit  int
	Offset int
}

// IssueQuery is an IssueFilter as the store runs it: checked, with the
// Projects the person may view (Issues without a Project are always kept)
// and the Guild's Issue prefix to search identifiers by. A Limit of 0 is
// no limit.
type IssueQuery struct {
	Statuses        []domain.IssueStatus
	Priorities      []domain.Priority
	AssigneeID      *uint64
	AssigneeAgentID *uint64
	ProjectID       *uint64
	GoalID          *uint64
	ParentID        *uint64
	TouchedBy       *uint64
	UnreadFor       *uint64
	InboxFor        *uint64
	Search          string
	Limit           int
	Offset          int
	Visible         []uint64
	Prefix          string
}

// InboxMember is the Member whose Inbox the filter asks about, or 0 when
// it asks about none.
func (f IssueFilter) InboxMember() uint64 {
	for _, m := range []*uint64{f.InboxFor, f.UnreadFor, f.TouchedBy} {
		if m != nil {
			return *m
		}
	}
	return 0
}

// InboxState is where an Issue stands in a Member's Inbox. LastTouchedAt
// is nil when the Member never touched it; Archived means it is in their
// Inbox archive and has not Resurfaced.
type InboxState struct {
	Unread        bool
	LastTouchedAt *time.Time
	Archived      bool
}

// IssueInput is a new Issue as typed. Empty Status and Priority take their
// defaults; the ids are 0 for none.
type IssueInput struct {
	Title       string
	Description string
	Status      string
	Priority    string
	AssigneeID  uint64
	// AssigneeAgentID is an Agent Assignee, instead of a Member.
	AssigneeAgentID uint64
	ProjectID       uint64
	GoalID          uint64
	ParentID        uint64
}

// IssuePatch changes the fields that are not nil. An id of 0 removes the
// Assignee, Project, Goal or parent.
type IssuePatch struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	AssigneeID  *uint64
	// AssigneeAgentID is an Agent Assignee; it and AssigneeID may not both
	// be set to one.
	AssigneeAgentID *uint64
	ProjectID       *uint64
	GoalID          *uint64
	ParentID        *uint64
	// BlockedByIDs replaces the Blockers the person can see; empty removes
	// them.
	BlockedByIDs *[]uint64
}

// IssuePrefix is the Guild's Issue prefix, for its Issue identifiers.
func (s *Service) IssuePrefix(ctx context.Context, guildID uint64) (string, error) {
	return s.guilds.IssuePrefix(ctx, guildID)
}

// Issues lists the Guild's Issues the filter keeps and the person may see,
// most recently updated first.
func (s *Service) Issues(ctx context.Context, guildID uint64, f IssueFilter, visible Visible) ([]domain.Issue, error) {
	q := IssueQuery{
		AssigneeID: f.AssigneeID, AssigneeAgentID: f.AssigneeAgentID, ProjectID: f.ProjectID, GoalID: f.GoalID, ParentID: f.ParentID,
		TouchedBy: f.TouchedBy, UnreadFor: f.UnreadFor, InboxFor: f.InboxFor,
		Search: strings.TrimSpace(f.Search), Limit: f.Limit, Offset: max(f.Offset, 0),
	}
	for _, st := range f.Statuses {
		v, err := domain.ParseIssueStatus(st)
		if err != nil {
			return nil, err
		}
		q.Statuses = append(q.Statuses, v)
	}
	for _, p := range f.Priorities {
		v, err := domain.ParsePriority(p)
		if err != nil {
			return nil, err
		}
		q.Priorities = append(q.Priorities, v)
	}
	if q.Limit <= 0 || q.Limit > MaxIssues {
		q.Limit = MaxIssues
	}
	return s.findIssues(ctx, guildID, q, visible)
}

func (s *Service) findIssues(ctx context.Context, guildID uint64, q IssueQuery, visible Visible) ([]domain.Issue, error) {
	if err := s.scope(ctx, guildID, &q, visible); err != nil {
		return nil, err
	}
	return s.issues.FindIssues(ctx, guildID, q)
}

// scope gives q the Projects the person may view and the Guild's Issue
// prefix.
func (s *Service) scope(ctx context.Context, guildID uint64, q *IssueQuery, visible Visible) error {
	in, err := s.issues.IssueProjects(ctx, guildID)
	if err != nil {
		return err
	}
	if len(in) > 0 {
		if q.Visible, err = visible(in); err != nil {
			return err
		}
	}
	q.Prefix, err = s.guilds.IssuePrefix(ctx, guildID)
	return err
}

// InboxCount counts the Unread Issues in the Member's Mine tab that the
// person may see: the sidebar's Inbox badge.
func (s *Service) InboxCount(ctx context.Context, guildID, memberID uint64, visible Visible) (int64, error) {
	q := IssueQuery{InboxFor: &memberID, UnreadFor: &memberID}
	if err := s.scope(ctx, guildID, &q, visible); err != nil {
		return 0, err
	}
	return s.issues.CountIssues(ctx, guildID, q)
}

// InboxStates is where each Issue stands in the Member's Inbox.
func (s *Service) InboxStates(ctx context.Context, memberID uint64, is []domain.Issue) (map[uint64]InboxState, error) {
	if len(is) == 0 {
		return map[uint64]InboxState{}, nil
	}
	ids := make([]uint64, len(is))
	for n, i := range is {
		ids[n] = i.ID
	}
	return s.issues.InboxStates(ctx, memberID, ids)
}

// SubIssues lists the Issue's Sub-issues the person may see, unpaged.
func (s *Service) SubIssues(ctx context.Context, i domain.Issue, visible Visible) ([]domain.Issue, error) {
	return s.findIssues(ctx, i.GuildID, IssueQuery{ParentID: &i.ID}, visible)
}

// GoalIssues lists the Issues serving the Goal that the person may see,
// unpaged.
func (s *Service) GoalIssues(ctx context.Context, g domain.Goal, visible Visible) ([]domain.Issue, error) {
	return s.findIssues(ctx, g.GuildID, IssueQuery{GoalID: &g.ID}, visible)
}

// Issue finds one of the Guild's Issues by its id or its Issue identifier
// (DEF-12, any case). Another Guild's Issue, or one in a Project the
// person may not view, is ErrNotFound.
func (s *Service) Issue(ctx context.Context, guildID uint64, ref string, visible Visible) (domain.Issue, error) {
	var (
		i     domain.Issue
		found bool
		err   error
	)
	if id, perr := strconv.ParseUint(ref, 10, 64); perr == nil {
		i, found, err = s.issues.Issue(ctx, id)
	} else if prefix, number, ok := domain.ParseIdentifier(ref); ok {
		own, perr := s.guilds.IssuePrefix(ctx, guildID)
		if perr != nil {
			return domain.Issue{}, perr
		}
		if strings.EqualFold(prefix, own) {
			i, found, err = s.issues.IssueByNumber(ctx, guildID, number)
		}
	}
	if err != nil {
		return domain.Issue{}, err
	}
	if !found || i.GuildID != guildID {
		return domain.Issue{}, ErrNotFound
	}
	if ok, err := s.mayView(i.ProjectID, visible); err != nil || !ok {
		if err == nil {
			err = ErrNotFound
		}
		return domain.Issue{}, err
	}
	return i, nil
}

// IssueOfGuild is the Guild's Issue by id with the Guild's Issue prefix,
// whoever may view it, for another context; found is false when it is
// another Guild's.
func (s *Service) IssueOfGuild(ctx context.Context, guildID, id uint64) (i domain.Issue, prefix string, found bool, err error) {
	i, found, err = s.issues.Issue(ctx, id)
	if err != nil || !found || i.GuildID != guildID {
		return domain.Issue{}, "", false, err
	}
	if prefix, err = s.guilds.IssuePrefix(ctx, guildID); err != nil {
		return domain.Issue{}, "", false, err
	}
	return i, prefix, true, nil
}

// VisibleIssues returns the Issues with these ids that the person may see,
// for the parents an Issue names.
func (s *Service) VisibleIssues(ctx context.Context, ids []uint64, visible Visible) ([]domain.Issue, error) {
	is, err := s.issues.IssuesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := is[:0]
	for _, i := range is {
		ok, err := s.mayView(i.ProjectID, visible)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, i)
		}
	}
	return out, nil
}

func (s *Service) mayView(projectID uint64, visible Visible) (bool, error) {
	if projectID == 0 {
		return true, nil
	}
	ids, err := visible([]uint64{projectID})
	return len(ids) == 1, err
}

// CreateIssue creates an Issue in the Guild by the Member.
func (s *Service) CreateIssue(ctx context.Context, guildID, memberID uint64, in IssueInput, visible Visible) (domain.Issue, error) {
	i, err := domain.NewIssue(guildID, memberID, in.Title, in.Description)
	if err != nil {
		return domain.Issue{}, err
	}
	if in.Status != "" {
		st, err := domain.ParseIssueStatus(in.Status)
		if err != nil {
			return domain.Issue{}, err
		}
		i.SetStatus(st, s.now())
	}
	if in.Priority != "" {
		p, err := domain.ParsePriority(in.Priority)
		if err != nil {
			return domain.Issue{}, err
		}
		i.SetPriority(p)
	}
	if err := s.assignOne(ctx, &i, &in.AssigneeID, &in.AssigneeAgentID); err != nil {
		return domain.Issue{}, err
	}
	if err := s.relate(ctx, &i, in.ProjectID, in.GoalID, in.ParentID, visible); err != nil {
		return domain.Issue{}, err
	}
	if i, err = s.issues.CreateIssue(ctx, i); err != nil {
		return domain.Issue{}, err
	}
	s.publish(ctx, domain.IssueCreated{Happened: s.happened(memberID), Issue: i})
	if i.AssigneeAgentID != 0 && agentWorksOn(i.Status) {
		s.assigned(ctx, i, memberID)
	}
	return i, nil
}

// agentWorksOn tells whether an Agent assignee has work in an Issue with
// the status: it is open and out of the backlog.
func agentWorksOn(st domain.IssueStatus) bool {
	return st != domain.Backlog && st != domain.Done && st != domain.IssueCancelled
}

// assigned tells Assigned about an Issue that is stored. Its error is
// logged: the change it follows already happened.
func (s *Service) assigned(ctx context.Context, i domain.Issue, actorID uint64) {
	if s.Assigned == nil {
		return
	}
	if err := s.Assigned(ctx, i, actorID); err != nil {
		s.Logf("work: after assigning issue %d to agent %d: %v", i.ID, i.AssigneeAgentID, err)
	}
}

// relate sets an Issue's Project, Goal and parent, each checked.
func (s *Service) relate(ctx context.Context, i *domain.Issue, projectID, goalID, parentID uint64, visible Visible) error {
	if err := s.placeIn(ctx, i, projectID, visible); err != nil {
		return err
	}
	if err := s.serveGoal(ctx, i, goalID); err != nil {
		return err
	}
	return s.moveIssueUnder(ctx, i, parentID, visible)
}

// ChangeIssue changes the Issue by the Member.
func (s *Service) ChangeIssue(ctx context.Context, guildID, memberID uint64, ref string, p IssuePatch, visible Visible) (domain.Issue, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.Issue{}, err
	}
	e := domain.IssueChanged{Happened: s.happened(memberID), Before: i}
	if p.Title != nil {
		if err := i.Rename(*p.Title); err != nil {
			return domain.Issue{}, err
		}
	}
	if p.Description != nil {
		i.Describe(*p.Description)
	}
	if p.Status != nil {
		st, err := domain.ParseIssueStatus(*p.Status)
		if err != nil {
			return domain.Issue{}, err
		}
		i.SetStatus(st, s.now())
	}
	if p.Priority != nil {
		pr, err := domain.ParsePriority(*p.Priority)
		if err != nil {
			return domain.Issue{}, err
		}
		i.SetPriority(pr)
	}
	if err := s.assignOne(ctx, &i, p.AssigneeID, p.AssigneeAgentID); err != nil {
		return domain.Issue{}, err
	}
	if p.ProjectID != nil {
		if err := s.placeIn(ctx, &i, *p.ProjectID, visible); err != nil {
			return domain.Issue{}, err
		}
	}
	if p.GoalID != nil {
		if err := s.serveGoal(ctx, &i, *p.GoalID); err != nil {
			return domain.Issue{}, err
		}
	}
	if p.ParentID != nil {
		if err := s.moveIssueUnder(ctx, &i, *p.ParentID, visible); err != nil {
			return domain.Issue{}, err
		}
	}
	if p.BlockedByIDs != nil {
		if e.BlockersBefore, err = s.visibleBlockers(ctx, i, visible); err != nil {
			return domain.Issue{}, err
		}
		blockedBy, err := s.blockWith(ctx, i, *p.BlockedByIDs, visible)
		if err != nil {
			return domain.Issue{}, err
		}
		e.BlockersAfter = *p.BlockedByIDs
		err = s.issues.SaveIssueBlockedBy(ctx, i, blockedBy, memberID)
	} else {
		err = s.issues.SaveIssue(ctx, i)
	}
	if err != nil {
		return domain.Issue{}, err
	}
	if e.After, _, err = s.issues.Issue(ctx, i.ID); err != nil {
		return domain.Issue{}, err
	}
	if len(e.Changes()) > 0 {
		s.publish(ctx, e)
	}
	if a := e.After; a.AssigneeAgentID != 0 && agentWorksOn(a.Status) &&
		(a.AssigneeAgentID != e.Before.AssigneeAgentID || e.Before.Status == domain.Backlog) {
		s.assigned(ctx, a, memberID)
	}
	return e.After, nil
}

// visibleBlockers is the ids of the Issue's Blockers the person may see.
func (s *Service) visibleBlockers(ctx context.Context, i domain.Issue, visible Visible) ([]uint64, error) {
	have, err := s.issues.Blockers(ctx, []uint64{i.ID})
	if err != nil {
		return nil, err
	}
	shown, err := s.VisibleIssues(ctx, have[i.ID], visible)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(shown))
	for n, b := range shown {
		ids[n] = b.ID
	}
	return ids, nil
}

// blockWith checks ids as the Blockers the person sets on the Issue, each
// an Issue they must be able to see, and returns the Issue's whole new set:
// those, and the Blockers it has that the person cannot see.
func (s *Service) blockWith(ctx context.Context, i domain.Issue, ids []uint64, visible Visible) ([]uint64, error) {
	notFound := &domain.FieldError{Field: "blocked_by_ids", Message: "blocked-by issue not found"}
	found, err := s.issues.IssuesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]domain.Issue, len(found))
	for _, b := range found {
		byID[b.ID] = b
	}
	blockers := make([]domain.Issue, 0, len(ids))
	for _, id := range ids {
		b, ok := byID[id]
		if !ok {
			return nil, notFound
		}
		if b.GuildID == i.GuildID {
			if ok, err = s.mayView(b.ProjectID, visible); err != nil {
				return nil, err
			}
		}
		if !ok {
			return nil, notFound
		}
		blockers = append(blockers, b)
	}
	reachable, err := s.issues.BlockedFrom(ctx, i.ID)
	if err != nil {
		return nil, err
	}
	set, err := i.BlockWith(blockers, reachable)
	if err != nil {
		return nil, err
	}
	have, err := s.issues.Blockers(ctx, []uint64{i.ID})
	if err != nil {
		return nil, err
	}
	shown, err := s.visibleBlockers(ctx, i, visible)
	if err != nil {
		return nil, err
	}
	for _, id := range have[i.ID] {
		if !slices.Contains(shown, id) && !slices.Contains(set, id) {
			set = append(set, id)
		}
	}
	return set, nil
}

// Blockers returns, for each Issue, the Issues it is blocked by and the
// Issues it is blocking that the person may see, by number.
func (s *Service) Blockers(ctx context.Context, is []domain.Issue, visible Visible) (blockedBy, blocking map[uint64][]domain.Issue, err error) {
	ids := make([]uint64, len(is))
	for n, i := range is {
		ids[n] = i.ID
	}
	by, err := s.issues.Blockers(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	ing, err := s.issues.Blocking(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	var all []uint64
	for _, m := range []map[uint64][]uint64{by, ing} {
		for _, v := range m {
			all = append(all, v...)
		}
	}
	shown, err := s.VisibleIssues(ctx, all, visible)
	if err != nil {
		return nil, nil, err
	}
	slices.SortFunc(shown, func(a, b domain.Issue) int { return a.Number - b.Number })
	pick := func(m map[uint64][]uint64) map[uint64][]domain.Issue {
		out := make(map[uint64][]domain.Issue, len(m))
		for id, of := range m {
			for _, v := range shown {
				if slices.Contains(of, v.ID) {
					out[id] = append(out[id], v)
				}
			}
		}
		return out
	}
	return pick(by), pick(ing), nil
}

// DeleteIssue deletes the Issue by the Member; its Sub-issues lose their
// parent.
func (s *Service) DeleteIssue(ctx context.Context, guildID, memberID uint64, ref string, visible Visible) error {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return err
	}
	if err := s.issues.DeleteIssue(ctx, i.ID); err != nil {
		return err
	}
	s.publish(ctx, domain.IssueDeleted{Happened: s.happened(memberID), Issue: i})
	return nil
}

// ForgetProject takes every Issue, and every Activity event, out of a
// Project that was deleted.
func (s *Service) ForgetProject(ctx context.Context, projectID uint64) error {
	return s.issues.LeaveProject(ctx, projectID)
}

// HasIssues reports whether the Guild has any Issue.
func (s *Service) HasIssues(ctx context.Context, guildID uint64) (bool, error) {
	return s.issues.HasIssues(ctx, guildID)
}

// ProjectNames names the Guild's Projects among ids.
func (s *Service) ProjectNames(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]string, error) {
	return s.projects.ProjectNames(ctx, guildID, ids)
}

// assignOne hands the Issue to the Member or the Agent that is set and not
// 0; a nil one is left as it is, a 0 one taken off. Both set to one is
// refused, as an Issue has one Assignee.
func (s *Service) assignOne(ctx context.Context, i *domain.Issue, memberID, agentID *uint64) error {
	if memberID != nil && agentID != nil && *memberID != 0 && *agentID != 0 {
		return &domain.FieldError{Field: "assignee_agent_id", Message: "an issue is assigned to a member or an agent, not both"}
	}
	if memberID != nil {
		if err := s.assign(ctx, i, *memberID); err != nil {
			return err
		}
	}
	if agentID != nil {
		return s.assignAgent(ctx, i, *agentID)
	}
	return nil
}

// assignAgent hands the Issue to one of the Guild's Agents that is not
// terminated.
func (s *Service) assignAgent(ctx context.Context, i *domain.Issue, agentID uint64) error {
	if agentID != 0 {
		a, err := s.AssigneeAgents(ctx, i.GuildID, []uint64{agentID})
		if err != nil {
			return err
		}
		if found, ok := a[agentID]; !ok || found.Terminated {
			return &domain.FieldError{Field: "assignee_agent_id", Message: "assignee must be an agent of this guild that is not terminated"}
		}
	}
	i.AssignAgent(agentID)
	return nil
}

// AssigneeAgents names the Guild's Agents among ids, terminated ones too;
// an id that is not one of them is left out.
func (s *Service) AssigneeAgents(ctx context.Context, guildID uint64, ids []uint64) (map[uint64]AssigneeAgent, error) {
	if s.Agents == nil || len(ids) == 0 {
		return map[uint64]AssigneeAgent{}, nil
	}
	return s.Agents(ctx, guildID, ids)
}

// AgentWork lists the Guild's Issues the Agent is the assignee of that are
// todo, in progress or in review, by number, with the Guild's Issue
// prefix: what a Heartbeat tells the Agent to work on.
func (s *Service) AgentWork(ctx context.Context, guildID, agentID uint64) (is []domain.Issue, prefix string, err error) {
	all, err := s.issues.OpenIssuesOfAgent(ctx, guildID, agentID)
	if err != nil {
		return nil, "", err
	}
	for _, i := range all {
		if i.Status == domain.Todo || i.Status == domain.InProgress || i.Status == domain.InReview {
			is = append(is, i)
		}
	}
	if len(is) == 0 {
		return nil, "", nil
	}
	slices.SortFunc(is, func(a, b domain.Issue) int { return a.Number - b.Number })
	prefix, err = s.guilds.IssuePrefix(ctx, guildID)
	return is, prefix, err
}

// UnassignAgent takes a terminated Agent off the Guild's Issues that are
// not done or cancelled, each change recorded with the actor who
// terminated it.
func (s *Service) UnassignAgent(ctx context.Context, guildID, agentID, actorID uint64) error {
	is, err := s.issues.OpenIssuesOfAgent(ctx, guildID, agentID)
	if err != nil {
		return err
	}
	for _, i := range is {
		e := domain.IssueChanged{Happened: s.happened(actorID), Before: i}
		i.AssignAgent(0)
		if err := s.issues.SaveIssue(ctx, i); err != nil {
			return err
		}
		if e.After, _, err = s.issues.Issue(ctx, i.ID); err != nil {
			return err
		}
		s.publish(ctx, e)
	}
	return nil
}

func (s *Service) assign(ctx context.Context, i *domain.Issue, memberID uint64) error {
	if memberID != 0 {
		ok, err := s.guilds.IsMember(ctx, i.GuildID, memberID)
		if err != nil {
			return err
		}
		if !ok {
			return &domain.FieldError{Field: "assignee_id", Message: "assignee must be a member of this guild"}
		}
	}
	i.Assign(memberID)
	return nil
}

// placeIn puts the Issue in one of the Guild's Projects, which the person
// must be able to view.
func (s *Service) placeIn(ctx context.Context, i *domain.Issue, projectID uint64, visible Visible) error {
	if projectID != 0 {
		names, err := s.projects.ProjectNames(ctx, i.GuildID, []uint64{projectID})
		if err != nil {
			return err
		}
		ok := false
		if _, named := names[projectID]; named {
			if ok, err = s.mayView(projectID, visible); err != nil {
				return err
			}
		}
		if !ok {
			return &domain.FieldError{Field: "project_id", Message: "project not found"}
		}
	}
	i.PlaceIn(projectID)
	return nil
}

func (s *Service) serveGoal(ctx context.Context, i *domain.Issue, goalID uint64) error {
	if goalID == 0 {
		return i.ServeGoal(nil)
	}
	g, found, err := s.goals.Goal(ctx, goalID)
	if err != nil {
		return err
	}
	if !found {
		return &domain.FieldError{Field: "goal_id", Message: "goal not found"}
	}
	return i.ServeGoal(&g)
}

// moveIssueUnder looks up the parent Issue, which the person must be able
// to see, and its ancestors, and hands them to the Issue to check; 0
// removes the parent.
func (s *Service) moveIssueUnder(ctx context.Context, i *domain.Issue, parentID uint64, visible Visible) error {
	if parentID == 0 {
		return i.MoveUnder(nil, nil)
	}
	notFound := &domain.FieldError{Field: "parent_id", Message: "parent issue not found"}
	parent, found, err := s.issues.Issue(ctx, parentID)
	if err != nil {
		return err
	}
	if !found || parent.GuildID != i.GuildID {
		return notFound
	}
	if ok, err := s.mayView(parent.ProjectID, visible); err != nil || !ok {
		if err == nil {
			err = notFound
		}
		return err
	}
	// A cycle can only come back to i through stored parents; the walk
	// stops at a repeat so a broken chain cannot loop.
	var ancestors []uint64
	seen := map[uint64]bool{parent.ID: true}
	for up := parent.ParentID; up != 0 && !seen[up]; {
		seen[up] = true
		ancestors = append(ancestors, up)
		a, found, err := s.issues.Issue(ctx, up)
		if err != nil {
			return err
		}
		if !found {
			break
		}
		up = a.ParentID
	}
	return i.MoveUnder(&parent, ancestors)
}
