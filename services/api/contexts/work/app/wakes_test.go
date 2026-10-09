package app

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type (
	issuesPort   = Issues
	commentsPort = Comments
)

// memIssues keeps Issues in memory.
type memIssues struct {
	issuesPort
	byID map[uint64]domain.Issue
}

func (m *memIssues) CreateIssue(_ context.Context, i domain.Issue) (domain.Issue, error) {
	i.ID = uint64(len(m.byID) + 1)
	i.Number = int(i.ID)
	m.byID[i.ID] = i
	return i, nil
}

func (m *memIssues) Issue(_ context.Context, id uint64) (domain.Issue, bool, error) {
	i, ok := m.byID[id]
	return i, ok, nil
}

func (m *memIssues) IssueByNumber(_ context.Context, guildID uint64, number int) (domain.Issue, bool, error) {
	for _, i := range m.byID {
		if i.GuildID == guildID && i.Number == number {
			return i, true, nil
		}
	}
	return domain.Issue{}, false, nil
}

func (m *memIssues) SaveIssue(_ context.Context, i domain.Issue) error {
	m.byID[i.ID] = i
	return nil
}

func (m *memIssues) OpenIssuesOfAgent(_ context.Context, guildID, agentID uint64) ([]domain.Issue, error) {
	var out []domain.Issue
	for _, i := range m.byID {
		if i.GuildID == guildID && i.AssigneeAgentID == agentID && i.Conversation == nil && i.Status != domain.Done && i.Status != domain.IssueCancelled {
			out = append(out, i)
		}
	}
	return out, nil
}

func (m *memIssues) OpenExecutionIssues(_ context.Context, routineID uint64) ([]domain.Issue, error) {
	var out []domain.Issue
	for id := uint64(len(m.byID)); id > 0; id-- {
		if i, ok := m.byID[id]; ok && i.OriginRoutineID == routineID && i.Status != domain.Done && i.Status != domain.IssueCancelled {
			out = append(out, i)
		}
	}
	return out, nil
}

func (m *memIssues) DeleteIssue(_ context.Context, id uint64) error {
	delete(m.byID, id)
	return nil
}

// memComments keeps Comments in memory.
type memComments struct {
	commentsPort
	byID map[uint64]domain.Comment
	// issues, when set, takes a Conversation's moves.
	issues *memIssues
}

func (m *memComments) CreateComment(_ context.Context, c domain.Comment) (domain.Comment, error) {
	c.ID = uint64(len(m.byID) + 1)
	m.byID[c.ID] = c
	return c, nil
}

func (m *memComments) SaveComment(_ context.Context, c domain.Comment) (domain.Comment, error) {
	m.byID[c.ID] = c
	return c, nil
}

func (m *memComments) Comment(_ context.Context, id uint64) (domain.Comment, bool, error) {
	c, ok := m.byID[id]
	return c, ok, nil
}

// memberGuild has Members 7 and 8 and the Issue prefix BAK.
type memberGuild struct{}

func (memberGuild) IsMember(_ context.Context, _, memberID uint64) (bool, error) {
	return memberID == 7 || memberID == 8, nil
}

func (memberGuild) IssuePrefix(context.Context, uint64) (string, error) { return "BAK", nil }

// woken is what the hooks heard: an Issue id and, for a Comment, its id.
type woken struct{ issue, comment, agent uint64 }

func wakeService(t *testing.T) (*Service, *[]woken) {
	t.Helper()
	s := NewService(nil, &memIssues{byID: map[uint64]domain.Issue{}}, &memComments{byID: map[uint64]domain.Comment{}}, nil, memberGuild{}, nil, &memActivity{}, nil, nil, nil)
	s.Logf = t.Logf
	s.Agents = func(_ context.Context, _ uint64, ids []uint64) (map[uint64]AssigneeAgent, error) {
		return map[uint64]AssigneeAgent{ids[0]: {Name: "Ada"}}, nil
	}
	var heard []woken
	s.Assigned = func(_ context.Context, i domain.Issue, actorID uint64) error {
		if actorID != 7 || i.AssigneeAgentID != 3 {
			t.Errorf("assigned %+v by %d", i, actorID)
		}
		heard = append(heard, woken{issue: i.ID})
		return nil
	}
	s.Commented = func(_ context.Context, i domain.Issue, c domain.Comment, _ uint64) error {
		heard = append(heard, woken{issue: i.ID, comment: c.ID})
		return nil
	}
	return s, &heard
}

func ptr[T any](v T) *T { return &v }

func TestIssueAssignedHook(t *testing.T) {
	ctx := context.Background()
	s, heard := wakeService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	// Assigned to an Agent while todo: heard.
	todo, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Fix it", Status: "todo", AssigneeAgentID: 3}, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(*heard) != 1 || (*heard)[0].issue != todo.ID {
		t.Fatalf("created todo: %v", *heard)
	}
	// Created in the backlog: not heard until it leaves the backlog.
	backlog, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Later", Status: "backlog", AssigneeAgentID: 3}, all)
	if err != nil || len(*heard) != 1 {
		t.Fatalf("created in backlog: %v %v", err, *heard)
	}
	ref := func(i domain.Issue) string { return strconv.FormatUint(i.ID, 10) }
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref(backlog), IssuePatch{Title: ptr("Later, renamed")}, all); err != nil || len(*heard) != 1 {
		t.Fatalf("renamed in backlog: %v %v", err, *heard)
	}
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref(backlog), IssuePatch{Status: ptr("todo")}, all); err != nil || len(*heard) != 2 || (*heard)[1].issue != backlog.ID {
		t.Fatalf("left the backlog: %v %v", err, *heard)
	}
	// A change that leaves the same Agent on an open Issue is not heard.
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref(todo), IssuePatch{Status: ptr("in_progress"), AssigneeAgentID: ptr(uint64(3))}, all); err != nil || len(*heard) != 2 {
		t.Fatalf("same agent: %v %v", err, *heard)
	}
	// A Member as the Assignee is not heard; handing it to the Agent is.
	member, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Mine", Status: "todo", AssigneeID: 8}, all)
	if err != nil || len(*heard) != 2 {
		t.Fatalf("assigned to a member: %v %v", err, *heard)
	}
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref(member), IssuePatch{AssigneeID: ptr(uint64(0)), AssigneeAgentID: ptr(uint64(3))}, all); err != nil || len(*heard) != 3 {
		t.Fatalf("handed to the agent: %v %v", err, *heard)
	}
	// Assigned while done: not heard.
	if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Old", Status: "done", AssigneeAgentID: 3}, all); err != nil || len(*heard) != 3 {
		t.Fatalf("done: %v %v", err, *heard)
	}
	// A failing hook is logged, and the change still succeeds.
	s.Assigned = func(context.Context, domain.Issue, uint64) error { return errors.New("agents down") }
	if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Anyway", Status: "todo", AssigneeAgentID: 3}, all); err != nil {
		t.Fatalf("hook error failed the change: %v", err)
	}
}

func TestIssueCommentedHook(t *testing.T) {
	ctx := context.Background()
	s, heard := wakeService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	agent, _ := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Fix it", Status: "todo", AssigneeAgentID: 3}, all)
	member, _ := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Mine", Status: "todo", AssigneeID: 8}, all)
	done, _ := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Old", Status: "done", AssigneeAgentID: 3}, all)
	*heard = nil
	c, err := s.WriteComment(ctx, 1, domain.ByMember(7), "1", "Also the link.", all)
	if err != nil || len(*heard) != 1 || (*heard)[0] != (woken{issue: agent.ID, comment: c.ID}) {
		t.Fatalf("comment on the agent's issue: %v %v", err, *heard)
	}
	for _, i := range []domain.Issue{member, done} {
		if _, err := s.WriteComment(ctx, 1, domain.ByMember(7), strconv.FormatUint(i.ID, 10), "Hm.", all); err != nil || len(*heard) != 1 {
			t.Fatalf("comment on %s: %v %v", i.Title, err, *heard)
		}
	}
	// The Agent assignee's own Comment does not wake it; another Agent's does.
	if _, err := s.WriteComment(ctx, 1, domain.ByAgent(3), "1", "On it.", all); err != nil || len(*heard) != 1 {
		t.Fatalf("the agent's own comment: %v %v", err, *heard)
	}
	if _, err := s.WriteComment(ctx, 1, domain.ByAgent(4), "1", "Me too.", all); err != nil || len(*heard) != 2 {
		t.Fatalf("another agent's comment: %v %v", err, *heard)
	}
	*heard = (*heard)[:1]
	s.Commented = func(context.Context, domain.Issue, domain.Comment, uint64) error { return errors.New("agents down") }
	if _, err := s.WriteComment(ctx, 1, domain.ByMember(7), "1", "Anyway.", all); err != nil {
		t.Fatalf("hook error failed the comment: %v", err)
	}
	// The Comments a Run quotes: the Guild's, in order, deleted ones left out.
	if err := s.DeleteComment(ctx, 1, domain.ByMember(7), strconv.FormatUint(member.ID, 10), 2, all); err != nil {
		t.Fatal(err)
	}
	cs, err := s.CommentsOfGuild(ctx, 1, []uint64{4, 2, c.ID, 99})
	if err != nil || len(cs) != 2 || cs[0].ID != 4 || cs[1].ID != c.ID {
		t.Fatalf("comments of guild: %v %+v", err, cs)
	}
	if cs, _ := s.CommentsOfGuild(ctx, 2, []uint64{c.ID}); len(cs) != 0 {
		t.Errorf("another guild's comments: %+v", cs)
	}
}

func TestAgentWork(t *testing.T) {
	ctx := context.Background()
	s, _ := wakeService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	for _, st := range []string{"in_review", "backlog", "todo", "blocked", "done", "in_progress"} {
		if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: st, Status: st, AssigneeAgentID: 3}, all); err != nil {
			t.Fatal(err)
		}
	}
	is, prefix, err := s.AgentWork(ctx, 1, 3)
	if err != nil || prefix != "BAK" || len(is) != 3 || is[0].Title != "in_review" || is[1].Title != "todo" || is[2].Title != "in_progress" {
		t.Fatalf("agent work: %v %s %+v", err, prefix, is)
	}
}

func TestAgentInbox(t *testing.T) {
	ctx := context.Background()
	s, _ := wakeService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	for _, c := range []struct{ title, status, priority string }{
		{"blocked", "blocked", "critical"}, {"todo low", "todo", "low"}, {"backlog", "backlog", "high"},
		{"todo high", "todo", "high"}, {"done", "done", "high"}, {"review", "in_review", "low"}, {"working", "in_progress", "low"},
	} {
		if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: c.title, Status: c.status, Priority: c.priority, AssigneeAgentID: 3}, all); err != nil {
			t.Fatal(err)
		}
	}
	is, prefix, err := s.AgentInbox(ctx, 1, 3)
	if err != nil || prefix != "BAK" {
		t.Fatalf("agent inbox: %v %s", err, prefix)
	}
	var got []string
	for _, i := range is {
		got = append(got, i.Title)
	}
	if want := []string{"working", "review", "todo high", "todo low", "blocked"}; !slices.Equal(got, want) {
		t.Fatalf("agent inbox order: %v, want %v", got, want)
	}
	if is, _, _ := s.AgentWork(ctx, 1, 3); len(is) != 4 {
		t.Fatalf("a Heartbeat's work counts blocked Issues: %d", len(is))
	}
}
