package http

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// entityJSON is the Goal, Issue, Approval or Agent an Activity event is
// about: its current title (an Agent's name) while it exists, else the one
// the event kept.
type entityJSON struct {
	Type       string `json:"type"`
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier,omitempty"`
	Title      string `json:"title"`
	Exists     bool   `json:"exists"`
}

type activityJSON struct {
	ID         uint64         `json:"id"`
	Action     string         `json:"action"`
	Actor      *Member        `json:"actor"`
	ActorAgent *Agent         `json:"actor_agent"`
	Entity     entityJSON     `json:"entity"`
	Details    map[string]any `json:"details"`
	CreatedAt  time.Time      `json:"created_at"`
}

// idIn reads an id the details stored: a json.Number from the store, a
// uint64 from a fresh event; 0 for none.
func idIn(v any) uint64 {
	switch n := v.(type) {
	case json.Number:
		id, _ := strconv.ParseUint(n.String(), 10, 64)
		return id
	case uint64:
		return n
	case float64:
		return uint64(n)
	}
	return 0
}

// numberIn reads a number the details stored, such as an issue_number.
func numberIn(v any) int {
	return int(idIn(v))
}

// activityRefs is what the references in a page of Activity events are
// called now; a missing one no longer exists or may not be seen.
type activityRefs struct {
	prefix    string
	members   map[uint64]Member
	projects  map[uint64]string
	goals     map[uint64]string
	issues    map[uint64]domain.Issue
	approvals map[uint64]string
	routines  map[uint64]string
	// agents is nil when no one answers for Agents: every one counts as
	// existing under the name its event kept.
	agents map[uint64]string
	// skills is nil when no one answers for Skills: every one counts as
	// existing.
	skills map[uint64]string
	// assignees are the Agents Issues were assigned to and the Agents that
	// acted, terminated ones too.
	assignees map[uint64]app.AssigneeAgent
}

// assigneeIn reads one side of an assignee change: {id, kind} since Agents
// can be Assignees, a Member's bare id before; 0 for none.
func assigneeIn(v any) (uint64, string) {
	if m, ok := v.(map[string]any); ok {
		kind, _ := m["kind"].(string)
		return idIn(m["id"]), kind
	}
	return idIn(v), "member"
}

// assignee names one side of an assignee change, nil for none.
func (r activityRefs) assignee(v any) any {
	id, kind := assigneeIn(v)
	if id == 0 {
		return nil
	}
	out := map[string]any{"id": id, "name": nil, "kind": kind}
	if kind == "agent" {
		if a, ok := r.assignees[id]; ok {
			out["name"], out["icon"] = a.Name, a.Icon
		}
	} else if m, ok := r.members[id]; ok {
		out["name"] = m.Name
	}
	return out
}

func (r activityRefs) member(id uint64) map[string]any {
	if m, ok := r.members[id]; ok {
		return map[string]any{"id": id, "name": m.Name}
	}
	return map[string]any{"id": id, "name": nil}
}

func (r activityRefs) project(id uint64) map[string]any {
	if name, ok := r.projects[id]; ok {
		return map[string]any{"id": id, "name": name}
	}
	return map[string]any{"id": id, "name": nil}
}

func (r activityRefs) goal(id uint64) map[string]any {
	if title, ok := r.goals[id]; ok {
		return map[string]any{"id": id, "title": title}
	}
	return map[string]any{"id": id, "title": nil}
}

func (r activityRefs) issue(id uint64) map[string]any {
	if i, ok := r.issues[id]; ok {
		return map[string]any{"id": id, "identifier": domain.Identifier(r.prefix, i.Number), "title": i.Title}
	}
	return map[string]any{"id": id, "identifier": nil, "title": nil}
}

// changes is the event's details.changes, if it has them.
func changes(e domain.ActivityEvent) map[string]any {
	ch, _ := e.Details["changes"].(map[string]any)
	return ch
}

// fromTo calls f with the from and to ids of a reference change.
func fromTo(v any, f func(id uint64)) {
	c, _ := v.(map[string]any)
	for _, k := range []string{"from", "to"} {
		if id := idIn(c[k]); id != 0 {
			f(id)
		}
	}
}

// blockerIDs calls f with every id in a blockers change.
func blockerIDs(v any, f func(id uint64)) {
	c, _ := v.(map[string]any)
	for _, k := range []string{"added", "removed"} {
		list, _ := c[k].([]any)
		for _, id := range list {
			f(idIn(id))
		}
	}
}

// refKind is what a changed field refers to: a member, an assignee (a
// member or an agent), project, goal or issue; "" for a plain value. A Goal's parent is a Goal, an Issue's an
// Issue.
func refKind(entityType, field string) string {
	switch field {
	case "assignee":
		return "assignee"
	case "owner":
		return "member"
	case "project":
		return "project"
	case "goal":
		return "goal"
	case "parent":
		if entityType == domain.GoalEntity {
			return "goal"
		}
		return "issue"
	}
	return ""
}

// activityRefs asks for every name a page of events refers to, each kind
// in one go. Issues and Projects the person may not view count as gone.
func (c *Controller) activityRefs(ctx contractshttp.Context, es []domain.ActivityEvent) (activityRefs, error) {
	cx, guildID := ctx.Context(), c.guild(ctx)
	refs := activityRefs{members: map[uint64]Member{}, projects: map[uint64]string{}, goals: map[uint64]string{}, issues: map[uint64]domain.Issue{}, approvals: map[uint64]string{}}
	var err error
	if refs.prefix, err = c.service.IssuePrefix(cx, guildID); err != nil {
		return refs, err
	}
	var memberIDs, projectIDs, issueIDs, agentIDs, skillIDs, assigneeIDs []uint64
	withApprovals, withRoutines := false, false
	add := func(list *[]uint64) func(uint64) {
		return func(id uint64) {
			if id != 0 && !slices.Contains(*list, id) {
				*list = append(*list, id)
			}
		}
	}
	for _, e := range es {
		add(&memberIDs)(e.Actor.MemberID)
		add(&assigneeIDs)(e.Actor.AgentID)
		if e.EntityType == domain.IssueEntity {
			add(&issueIDs)(e.EntityID)
		}
		withApprovals = withApprovals || e.EntityType == domain.ApprovalEntity
		withRoutines = withRoutines || e.EntityType == domain.RoutineEntity
		if e.EntityType == domain.AgentEntity {
			add(&agentIDs)(e.EntityID)
		}
		if e.EntityType == domain.SkillEntity {
			add(&skillIDs)(e.EntityID)
		}
		for field, v := range changes(e) {
			switch refKind(e.EntityType, field) {
			case "member":
				fromTo(v, add(&memberIDs))
			case "assignee":
				c, _ := v.(map[string]any)
				for _, k := range []string{"from", "to"} {
					if id, kind := assigneeIn(c[k]); kind == "agent" {
						add(&assigneeIDs)(id)
					} else {
						add(&memberIDs)(id)
					}
				}
			case "project":
				fromTo(v, add(&projectIDs))
			case "issue":
				fromTo(v, add(&issueIDs))
			}
			if field == "blockers" {
				blockerIDs(v, add(&issueIDs))
			}
		}
	}
	if len(memberIDs) > 0 {
		ms, err := c.members(cx, memberIDs)
		if err != nil {
			return refs, err
		}
		for _, m := range ms {
			refs.members[m.ID] = m
		}
	}
	if len(projectIDs) > 0 {
		seen, err := c.Visible(ctx, projectIDs)
		if err != nil {
			return refs, err
		}
		if refs.projects, err = c.service.ProjectNames(cx, guildID, seen); err != nil {
			return refs, err
		}
	}
	gs, err := c.service.Goals(cx, guildID)
	if err != nil {
		return refs, err
	}
	for _, g := range gs {
		refs.goals[g.ID] = g.Title
	}
	if withApprovals {
		as, err := c.service.Approvals(cx, guildID, "")
		if err != nil {
			return refs, err
		}
		for _, a := range as {
			refs.approvals[a.ID] = a.Payload.Label()
		}
	}
	if withRoutines {
		rs, err := c.service.Routines(cx, guildID, app.RoutineFilter{}, c.visible(ctx))
		if err != nil {
			return refs, err
		}
		refs.routines = make(map[uint64]string, len(rs))
		for _, r := range rs {
			refs.routines[r.ID] = r.Title
		}
	}
	if len(skillIDs) > 0 && c.SkillNames != nil {
		if refs.skills, err = c.SkillNames(cx, guildID, skillIDs); err != nil {
			return refs, err
		}
	}
	if len(agentIDs) > 0 && c.AgentNames != nil {
		if refs.agents, err = c.AgentNames(cx, guildID, agentIDs); err != nil {
			return refs, err
		}
	}
	if refs.assignees, err = c.service.AssigneeAgents(cx, guildID, assigneeIDs); err != nil {
		return refs, err
	}
	is, err := c.service.VisibleIssues(cx, issueIDs, c.visible(ctx))
	if err != nil {
		return refs, err
	}
	for _, i := range is {
		if i.GuildID == guildID {
			refs.issues[i.ID] = i
		}
	}
	return refs, nil
}

// activityJSON shows events with their Actors, what they are about and
// the references in their changes by name.
func (c *Controller) activityJSON(ctx contractshttp.Context, es []domain.ActivityEvent) ([]activityJSON, error) {
	out := make([]activityJSON, len(es))
	if len(es) == 0 {
		return out, nil
	}
	refs, err := c.activityRefs(ctx, es)
	if err != nil {
		return nil, err
	}
	for n, e := range es {
		a := activityJSON{ID: e.ID, Action: e.Action, Details: e.Details, CreatedAt: e.CreatedAt}
		if m, ok := refs.members[e.Actor.MemberID]; ok {
			a.Actor = &m
		}
		if g, ok := refs.assignees[e.Actor.AgentID]; ok && e.Actor.AgentID != 0 {
			a.ActorAgent = &Agent{ID: e.Actor.AgentID, Name: g.Name, Icon: g.Icon}
		}
		a.Entity = entityJSON{Type: e.EntityType, ID: e.EntityID}
		switch e.EntityType {
		case domain.IssueEntity:
			a.Entity.Identifier = domain.Identifier(refs.prefix, numberIn(e.Details["issue_number"]))
			a.Entity.Title, _ = e.Details["issue_title"].(string)
			if i, ok := refs.issues[e.EntityID]; ok {
				a.Entity.Title, a.Entity.Exists = i.Title, true
			}
		case domain.GoalEntity:
			a.Entity.Title, _ = e.Details["title"].(string)
			if title, ok := refs.goals[e.EntityID]; ok {
				a.Entity.Title, a.Entity.Exists = title, true
			}
		case domain.ApprovalEntity:
			a.Entity.Title, _ = e.Details["title"].(string)
			if title, ok := refs.approvals[e.EntityID]; ok {
				a.Entity.Title, a.Entity.Exists = title, true
			}
		case domain.RoutineEntity:
			a.Entity.Title, _ = e.Details["title"].(string)
			if title, ok := refs.routines[e.EntityID]; ok {
				a.Entity.Title, a.Entity.Exists = title, true
			}
		case domain.BudgetEntity, domain.IncidentEntity:
			a.Entity.Title, _ = e.Details["name"].(string)
			a.Entity.Exists = true
		case domain.SkillEntity:
			a.Entity.Title, _ = e.Details["name"].(string)
			a.Entity.Exists = refs.skills == nil
			if name, ok := refs.skills[e.EntityID]; ok {
				a.Entity.Title, a.Entity.Exists = name, true
			}
		case domain.AgentEntity:
			a.Entity.Title, _ = e.Details["name"].(string)
			a.Entity.Exists = refs.agents == nil
			if name, ok := refs.agents[e.EntityID]; ok {
				a.Entity.Title, a.Entity.Exists = name, true
			}
		}
		if ch := changes(e); ch != nil {
			named := make(map[string]any, len(ch))
			for field, v := range ch {
				named[field] = refs.name(e.EntityType, field, v)
			}
			e.Details["changes"] = named
		}
		out[n] = a
	}
	return out, nil
}

// name answers one change with its references by name.
func (r activityRefs) name(entityType, field string, v any) any {
	var by func(uint64) map[string]any
	switch refKind(entityType, field) {
	case "assignee":
		c, _ := v.(map[string]any)
		return map[string]any{"from": r.assignee(c["from"]), "to": r.assignee(c["to"])}
	case "member":
		by = r.member
	case "project":
		by = r.project
	case "goal":
		by = r.goal
	case "issue":
		by = r.issue
	}
	if by != nil {
		c, _ := v.(map[string]any)
		out := map[string]any{}
		for _, k := range []string{"from", "to"} {
			out[k] = nil
			if id := idIn(c[k]); id != 0 {
				out[k] = by(id)
			}
		}
		return out
	}
	if field == "blockers" {
		c, _ := v.(map[string]any)
		out := map[string]any{}
		for _, k := range []string{"added", "removed"} {
			list, _ := c[k].([]any)
			named := make([]any, 0, len(list))
			for _, id := range list {
				named = append(named, r.issue(idIn(id)))
			}
			out[k] = named
		}
		return out
	}
	return v
}

// activityID reads an id from the query string: absent is 0, anything but
// an id is not ok.
func activityID(ctx contractshttp.Context, field string) (uint64, bool) {
	s := strings.TrimSpace(ctx.Request().Query(field))
	if s == "" {
		return 0, true
	}
	id, err := strconv.ParseUint(s, 10, 64)
	return id, err == nil && id != 0
}

// ListActivity answers a page of the Current guild's Activity, newest
// first: entity (issue, goal, approval, agent, budget, budget_incident or routine), entity_id (one of
// that kind, such as a Routine's events on its page), actor (a Member id, or
// agent:<id> for an Agent), before (an Activity
// event id, for the next page) and limit (1 to 200, 50 by default).
func (c *Controller) ListActivity(ctx contractshttp.Context) contractshttp.Response {
	q := app.ActivityQuery{EntityType: ctx.Request().Query("entity"), Limit: app.DefaultActivity}
	if q.EntityType != "" && !slices.Contains([]string{domain.IssueEntity, domain.GoalEntity, domain.ApprovalEntity, domain.AgentEntity, domain.BudgetEntity, domain.IncidentEntity, domain.RoutineEntity, domain.SkillEntity}, q.EntityType) {
		return respond.Invalid(ctx, "entity", "entity must be issue, goal, approval, agent, budget, budget_incident or routine")
	}
	entityID, ok := activityID(ctx, "entity_id")
	if !ok || (entityID != 0 && q.EntityType == "") {
		return respond.Invalid(ctx, "entity_id", "entity_id must be an id, with entity")
	}
	q.EntityID = entityID
	if a, ok := strings.CutPrefix(ctx.Request().Query("actor"), "agent:"); ok {
		id, err := strconv.ParseUint(a, 10, 64)
		if err != nil || id == 0 {
			return respond.Invalid(ctx, "actor", "actor must be a member id or agent:<id>")
		}
		q.Actor = domain.ByAgent(id)
	} else if id, ok := activityID(ctx, "actor"); !ok {
		return respond.Invalid(ctx, "actor", "actor must be a member id or agent:<id>")
	} else {
		q.Actor = domain.ByMember(id)
	}
	before, ok := activityID(ctx, "before")
	if !ok {
		return respond.Invalid(ctx, "before", "before must be an id")
	}
	q.Before = before
	if s := ctx.Request().Query("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > app.MaxActivity {
			return respond.Invalid(ctx, "limit", "limit must be between 1 and "+strconv.Itoa(app.MaxActivity))
		}
		q.Limit = v
	}
	es, err := c.service.Activity(ctx.Context(), c.guild(ctx), q, c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.activityJSON(ctx, es)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"activity": out})
}

// ListIssueActivity answers every event about the Issue, oldest first.
func (c *Controller) ListIssueActivity(ctx contractshttp.Context) contractshttp.Response {
	es, err := c.service.IssueActivity(ctx.Context(), c.guild(ctx), ctx.Request().Route("id"), c.visible(ctx))
	if err != nil {
		return fail(ctx, err)
	}
	out, err := c.activityJSON(ctx, es)
	if err != nil {
		return fail(ctx, err)
	}
	return ctx.Response().Success().Json(contractshttp.Json{"activity": out})
}
