package app

import (
	"context"
	"strconv"
	"strings"

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
	// DeleteIssue deletes the Issue; its Sub-issues lose their parent.
	DeleteIssue(ctx context.Context, id uint64) error
	// FindIssues lists the Guild's Issues that q keeps, most recently
	// updated first.
	FindIssues(ctx context.Context, guildID uint64, q IssueQuery) ([]domain.Issue, error)
	// IssueProjects lists the Projects the Guild's Issues are in.
	IssueProjects(ctx context.Context, guildID uint64) ([]uint64, error)
	// LeaveProject takes every Issue out of the Project.
	LeaveProject(ctx context.Context, projectID uint64) error
	HasIssues(ctx context.Context, guildID uint64) (bool, error)
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
	AssigneeID *uint64
	ProjectID  *uint64
	GoalID     *uint64
	ParentID   *uint64
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
	Statuses   []domain.IssueStatus
	Priorities []domain.Priority
	AssigneeID *uint64
	ProjectID  *uint64
	GoalID     *uint64
	ParentID   *uint64
	Search     string
	Limit      int
	Offset     int
	Visible    []uint64
	Prefix     string
}

// IssueInput is a new Issue as typed. Empty Status and Priority take their
// defaults; the ids are 0 for none.
type IssueInput struct {
	Title       string
	Description string
	Status      string
	Priority    string
	AssigneeID  uint64
	ProjectID   uint64
	GoalID      uint64
	ParentID    uint64
}

// IssuePatch changes the fields that are not nil. An id of 0 removes the
// Assignee, Project, Goal or parent.
type IssuePatch struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	AssigneeID  *uint64
	ProjectID   *uint64
	GoalID      *uint64
	ParentID    *uint64
}

// IssuePrefix is the Guild's Issue prefix, for its Issue identifiers.
func (s *Service) IssuePrefix(ctx context.Context, guildID uint64) (string, error) {
	return s.guilds.IssuePrefix(ctx, guildID)
}

// Issues lists the Guild's Issues the filter keeps and the person may see,
// most recently updated first.
func (s *Service) Issues(ctx context.Context, guildID uint64, f IssueFilter, visible Visible) ([]domain.Issue, error) {
	q := IssueQuery{
		AssigneeID: f.AssigneeID, ProjectID: f.ProjectID, GoalID: f.GoalID, ParentID: f.ParentID,
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
	in, err := s.issues.IssueProjects(ctx, guildID)
	if err != nil {
		return nil, err
	}
	if len(in) > 0 {
		if q.Visible, err = visible(in); err != nil {
			return nil, err
		}
	}
	if q.Prefix, err = s.guilds.IssuePrefix(ctx, guildID); err != nil {
		return nil, err
	}
	return s.issues.FindIssues(ctx, guildID, q)
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
	if err := s.relate(ctx, &i, in.AssigneeID, in.ProjectID, in.GoalID, in.ParentID, visible); err != nil {
		return domain.Issue{}, err
	}
	return s.issues.CreateIssue(ctx, i)
}

// relate sets an Issue's Assignee, Project, Goal and parent, each checked.
func (s *Service) relate(ctx context.Context, i *domain.Issue, assigneeID, projectID, goalID, parentID uint64, visible Visible) error {
	if err := s.assign(ctx, i, assigneeID); err != nil {
		return err
	}
	if err := s.placeIn(ctx, i, projectID, visible); err != nil {
		return err
	}
	if err := s.serveGoal(ctx, i, goalID); err != nil {
		return err
	}
	return s.moveIssueUnder(ctx, i, parentID, visible)
}

func (s *Service) ChangeIssue(ctx context.Context, guildID uint64, ref string, p IssuePatch, visible Visible) (domain.Issue, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.Issue{}, err
	}
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
	if p.AssigneeID != nil {
		if err := s.assign(ctx, &i, *p.AssigneeID); err != nil {
			return domain.Issue{}, err
		}
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
	if err := s.issues.SaveIssue(ctx, i); err != nil {
		return domain.Issue{}, err
	}
	i, _, err = s.issues.Issue(ctx, i.ID)
	return i, err
}

// DeleteIssue deletes the Issue; its Sub-issues lose their parent.
func (s *Service) DeleteIssue(ctx context.Context, guildID uint64, ref string, visible Visible) error {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return err
	}
	return s.issues.DeleteIssue(ctx, i.ID)
}

// ForgetProject takes every Issue out of a Project that was deleted.
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
