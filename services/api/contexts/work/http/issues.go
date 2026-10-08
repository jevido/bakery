package http

import (
	"strconv"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type projectJSON struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type goalRefJSON struct {
	ID    uint64 `json:"id"`
	Title string `json:"title"`
}

type issueRefJSON struct {
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// issueJSON is an Issue as a list row shows it; Description is nil where a
// row leaves it out (an Issue's sub-issues).
type issueJSON struct {
	ID          uint64        `json:"id"`
	Number      int           `json:"number"`
	Identifier  string        `json:"identifier"`
	Title       string        `json:"title"`
	Description *string       `json:"description,omitempty"`
	Status      string        `json:"status"`
	Priority    string        `json:"priority"`
	Assignee    *Member       `json:"assignee"`
	Project     *projectJSON  `json:"project"`
	Goal        *goalRefJSON  `json:"goal"`
	Parent      *issueRefJSON `json:"parent"`
	CreatedBy   *Member       `json:"created_by"`
	StartedAt   *time.Time    `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at"`
	CancelledAt *time.Time    `json:"cancelled_at"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	// UnresolvedBlockers counts the Blockers the person can see that are
	// not done; list rows only.
	UnresolvedBlockers *int `json:"unresolved_blockers,omitempty"`
	// Unread, LastTouchedAt and Archived are where the Issue stands in the
	// asking Member's Inbox; only when the list was asked with touched,
	// unread or inbox.
	Unread        *bool      `json:"unread,omitempty"`
	LastTouchedAt *time.Time `json:"last_touched_at,omitempty"`
	Archived      *bool      `json:"archived,omitempty"`
}

// blockerJSON is an Issue in another Issue's blocked_by or blocking.
type blockerJSON struct {
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

// visible is the request's guilds.VisibleProjects as the service asks it.
func (c *Controller) visible(ctx contractshttp.Context) app.Visible {
	return func(ids []uint64) ([]uint64, error) { return c.Visible(ctx, ids) }
}

// issuesJSON shows Issues with the names of their Members, Projects, Goals
// and parents, each kind asked for in one go.
func (c *Controller) issuesJSON(ctx contractshttp.Context, is []domain.Issue, withDescription bool) ([]issueJSON, error) {
	out := make([]issueJSON, len(is))
	if len(is) == 0 {
		return out, nil
	}
	cx, guildID := ctx.Context(), c.guild(ctx)
	prefix, err := c.service.IssuePrefix(cx, guildID)
	if err != nil {
		return nil, err
	}
	var memberIDs, projectIDs, parentIDs []uint64
	for _, i := range is {
		for _, id := range []uint64{i.AssigneeID, i.CreatedByID} {
			if id != 0 {
				memberIDs = append(memberIDs, id)
			}
		}
		if i.ProjectID != 0 {
			projectIDs = append(projectIDs, i.ProjectID)
		}
		if i.ParentID != 0 {
			parentIDs = append(parentIDs, i.ParentID)
		}
	}
	members := map[uint64]Member{}
	if len(memberIDs) > 0 {
		ms, err := c.members(cx, memberIDs)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			members[m.ID] = m
		}
	}
	projects, err := c.service.ProjectNames(cx, guildID, projectIDs)
	if err != nil {
		return nil, err
	}
	gs, err := c.service.Goals(cx, guildID)
	if err != nil {
		return nil, err
	}
	goals := make(map[uint64]string, len(gs))
	for _, g := range gs {
		goals[g.ID] = g.Title
	}
	ps, err := c.service.VisibleIssues(cx, parentIDs, c.visible(ctx))
	if err != nil {
		return nil, err
	}
	parents := make(map[uint64]issueRefJSON, len(ps))
	for _, p := range ps {
		parents[p.ID] = issueRefJSON{ID: p.ID, Identifier: domain.Identifier(prefix, p.Number), Title: p.Title}
	}
	for n, i := range is {
		out[n] = issueJSON{
			ID: i.ID, Number: i.Number, Identifier: domain.Identifier(prefix, i.Number), Title: i.Title,
			Status: string(i.Status), Priority: string(i.Priority),
			StartedAt: i.StartedAt, CompletedAt: i.CompletedAt, CancelledAt: i.CancelledAt,
			CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt,
		}
		if withDescription {
			d := i.Description
			out[n].Description = &d
		}
		if m, ok := members[i.AssigneeID]; ok {
			out[n].Assignee = &m
		}
		if m, ok := members[i.CreatedByID]; ok {
			out[n].CreatedBy = &m
		}
		if name, ok := projects[i.ProjectID]; ok {
			out[n].Project = &projectJSON{ID: i.ProjectID, Name: name}
		}
		if title, ok := goals[i.GoalID]; ok {
			out[n].Goal = &goalRefJSON{ID: i.GoalID, Title: title}
		}
		if p, ok := parents[i.ParentID]; ok {
			out[n].Parent = &p
		}
	}
	return out, nil
}

type issueRequest struct {
	Title       optional[string] `json:"title"`
	Description optional[string] `json:"description"`
	Status      optional[string] `json:"status"`
	Priority    optional[string] `json:"priority"`
	AssigneeID  optional[uint64] `json:"assignee_id"`
	ProjectID   optional[uint64] `json:"project_id"`
	GoalID      optional[uint64] `json:"goal_id"`
	ParentID    optional[uint64] `json:"parent_id"`
	// BlockedByIDs is PATCH only; null clears like [].
	BlockedByIDs optional[[]uint64] `json:"blocked_by_ids"`
}

func (r issueRequest) input() app.IssueInput {
	return app.IssueInput{
		Title: value(r.Title.ptr()), Description: value(r.Description.ptr()), Status: value(r.Status.ptr()), Priority: value(r.Priority.ptr()),
		AssigneeID: value(idOf(r.AssigneeID)), ProjectID: value(idOf(r.ProjectID)), GoalID: value(idOf(r.GoalID)), ParentID: value(idOf(r.ParentID)),
	}
}

func (r issueRequest) patch() app.IssuePatch {
	p := app.IssuePatch{
		Title: r.Title.ptr(), Description: r.Description.ptr(), Status: r.Status.ptr(), Priority: r.Priority.ptr(),
		AssigneeID: idOf(r.AssigneeID), ProjectID: idOf(r.ProjectID), GoalID: idOf(r.GoalID), ParentID: idOf(r.ParentID),
	}
	if r.BlockedByIDs.Set {
		ids := value(r.BlockedByIDs.Value)
		p.BlockedByIDs = &ids
	}
	if r.Description.Set && p.Description == nil {
		// A null description is an empty one.
		p.Description = new(string)
	}
	return p
}

// list reads a comma list from the query string; empty is none.
func list(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// idFilter reads an id filter from the query string: absent keeps every
// Issue, "none" those without one, else an id; me is the asking Member's
// id where "me" is allowed (0 where it is not).
func idFilter(ctx contractshttp.Context, field string, me uint64) (*uint64, bool) {
	s := strings.TrimSpace(ctx.Request().Query(field))
	var id uint64
	switch {
	case s == "":
		return nil, true
	case s == "none":
	case s == "me" && me != 0:
		id = me
	default:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil || v == 0 {
			return nil, false
		}
		id = v
	}
	return &id, true
}

func (c *Controller) filter(ctx contractshttp.Context) (app.IssueFilter, contractshttp.Response) {
	r := ctx.Request()
	f := app.IssueFilter{Statuses: list(r.Query("status")), Priorities: list(r.Query("priority")), Search: r.Query("q")}
	for _, idf := range []struct {
		field string
		me    uint64
		into  **uint64
	}{
		{"assignee", c.Member(ctx), &f.AssigneeID},
		{"project", 0, &f.ProjectID},
		{"goal", 0, &f.GoalID},
		{"parent", 0, &f.ParentID},
	} {
		id, ok := idFilter(ctx, idf.field, idf.me)
		if !ok {
			return f, respond.Invalid(ctx, idf.field, idf.field+" must be an id or none")
		}
		*idf.into = id
	}
	for _, mf := range []struct {
		field string
		into  **uint64
	}{{"touched", &f.TouchedBy}, {"unread", &f.UnreadFor}, {"inbox", &f.InboxFor}} {
		switch strings.TrimSpace(r.Query(mf.field)) {
		case "":
		case "me":
			m := c.Member(ctx)
			*mf.into = &m
		default:
			return f, respond.Invalid(ctx, mf.field, mf.field+" must be me")
		}
	}
	for _, n := range []struct {
		field string
		into  *int
	}{{"limit", &f.Limit}, {"offset", &f.Offset}} {
		if s := r.Query(n.field); s != "" {
			v, err := strconv.Atoi(s)
			if err != nil || v < 0 {
				return f, respond.Invalid(ctx, n.field, n.field+" must be a whole number")
			}
			*n.into = v
		}
	}
	if f.Limit > app.MaxIssues {
		return f, respond.Invalid(ctx, "limit", "limit is at most "+strconv.Itoa(app.MaxIssues))
	}
	return f, nil
}

// ListIssues answers the Current guild's Issues that the filters keep and
// the request may see, most recently updated first: status and priority
// (comma lists), assignee (an id, me or none), project, goal and parent
// (an id or none), touched, unread and inbox (only me: the asking
// Member's Inbox tabs), q (title, description, identifier), limit (at
// most 200, the default) and offset. With touched, unread or inbox each
// Issue also answers unread, last_touched_at and archived for the Member.
func (c *Controller) ListIssues(ctx contractshttp.Context) contractshttp.Response {
	f, bad := c.filter(ctx)
	if bad != nil {
		return bad
	}
	is, err := c.service.Issues(ctx.Context(), c.guild(ctx), f, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.issuesJSON(ctx, is, true)
	if err != nil {
		return fail(ctx, err)
	}
	blockedBy, _, err := c.service.Blockers(ctx.Context(), is, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	for n, i := range is {
		unresolved := 0
		for _, b := range blockedBy[i.ID] {
			if b.Status != domain.Done {
				unresolved++
			}
		}
		out[n].UnresolvedBlockers = &unresolved
	}
	if m := f.InboxMember(); m != 0 {
		states, err := c.service.InboxStates(ctx.Context(), m, is)
		if err != nil {
			return fail(ctx, err)
		}
		for n, i := range is {
			st := states[i.ID]
			out[n].Unread, out[n].LastTouchedAt, out[n].Archived = &st.Unread, st.LastTouchedAt, &st.Archived
		}
	}
	return ctx.Response().Success().Json(contractshttp.Json{"issues": out})
}

// oneIssue answers an Issue with its Sub-issues and its Blockers both ways.
func (c *Controller) oneIssue(ctx contractshttp.Context, status int, i domain.Issue, err error) contractshttp.Response {
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.issuesJSON(ctx, []domain.Issue{i}, true)
	if err != nil {
		return fail(ctx, err)
	}
	subs, err := c.service.SubIssues(ctx.Context(), i, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	children, err := c.issuesJSON(ctx, subs, false)
	if err != nil {
		return fail(ctx, err)
	}
	blockedBy, blocking, err := c.service.Blockers(ctx.Context(), []domain.Issue{i}, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	prefix, err := c.service.IssuePrefix(ctx.Context(), c.guild(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	refs := func(is []domain.Issue) []blockerJSON {
		out := make([]blockerJSON, len(is))
		for n, b := range is {
			out[n] = blockerJSON{ID: b.ID, Identifier: domain.Identifier(prefix, b.Number), Title: b.Title, Status: string(b.Status)}
		}
		return out
	}
	return ctx.Response().Json(status, contractshttp.Json{"issue": struct {
		issueJSON
		Children  []issueJSON   `json:"children"`
		BlockedBy []blockerJSON `json:"blocked_by"`
		Blocking  []blockerJSON `json:"blocking"`
	}{out[0], children, refs(blockedBy[i.ID]), refs(blocking[i.ID])}})
}

func (c *Controller) CreateIssue(ctx contractshttp.Context) contractshttp.Response {
	var req issueRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	i, err := c.service.CreateIssue(ctx.Context(), c.guild(ctx), c.Member(ctx), req.input(), c.visible(ctx))
	return c.oneIssue(ctx, contractshttp.StatusCreated, i, err)
}

// ShowIssue answers the Issue named by its id or its Issue identifier.
func (c *Controller) ShowIssue(ctx contractshttp.Context) contractshttp.Response {
	i, err := c.service.Issue(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), c.visible(ctx))
	return c.oneIssue(ctx, contractshttp.StatusOK, i, err)
}

func (c *Controller) UpdateIssue(ctx contractshttp.Context) contractshttp.Response {
	var req issueRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	i, err := c.service.ChangeIssue(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), req.patch(), c.visible(ctx))
	return c.oneIssue(ctx, contractshttp.StatusOK, i, err)
}

func (c *Controller) DeleteIssue(ctx contractshttp.Context) contractshttp.Response {
	if err := c.service.DeleteIssue(ctx.Context(), c.guild(ctx), c.Member(ctx), ctx.Request().Route("id"), c.visible(ctx)); err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().NoContent()
}

// goalIssues is a Goal's visible Issues as rows and how many there are of
// each Issue status.
func (c *Controller) goalIssues(ctx contractshttp.Context, g domain.Goal) ([]issueJSON, map[string]int, error) {
	is, err := c.service.GoalIssues(ctx.Context(), g, c.visible(ctx))
	if err != nil {
		return nil, nil, err
	}
	counts := make(map[string]int, len(domain.IssueStatuses))
	for _, st := range domain.IssueStatuses {
		counts[string(st)] = 0
	}
	for _, i := range is {
		counts[string(i.Status)]++
	}
	rows, err := c.issuesJSON(ctx, is, false)
	return rows, counts, err
}
