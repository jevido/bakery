package app

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

func (m *memIssues) ConversationOf(_ context.Context, guildID, memberID, agentID uint64) (domain.Issue, bool, error) {
	for _, i := range m.byID {
		if c := i.Conversation; i.GuildID == guildID && c != nil && c.MemberID == memberID && c.AgentID == agentID {
			return i, true, nil
		}
	}
	return domain.Issue{}, false, nil
}

func (m *memIssues) Conversations(_ context.Context, guildID, memberID uint64) ([]domain.Issue, error) {
	var out []domain.Issue
	for _, i := range m.byID {
		if i.GuildID == guildID && i.Conversation != nil && i.Conversation.MemberID == memberID {
			out = append(out, i)
		}
	}
	return out, nil
}

// FindIssues keeps the Guild's Issues by Conversation, Search and Agent
// assignee only: what the Conversation tests ask of it.
func (m *memIssues) FindIssues(_ context.Context, guildID uint64, q IssueQuery) ([]domain.Issue, error) {
	var out []domain.Issue
	for id := uint64(1); id <= uint64(len(m.byID)); id++ {
		i, ok := m.byID[id]
		switch {
		case !ok, i.GuildID != guildID, i.Conversation != nil && !q.WithConversations,
			q.Search != "" && !strings.Contains(i.Title, q.Search),
			q.AssigneeAgentID != nil && i.AssigneeAgentID != *q.AssigneeAgentID:
			continue
		}
		out = append(out, i)
	}
	return out, nil
}

func (m *memIssues) CountIssues(ctx context.Context, guildID uint64, q IssueQuery) (int64, error) {
	is, err := m.FindIssues(ctx, guildID, q)
	return int64(len(is)), err
}

func (m *memIssues) IssueProjects(context.Context, uint64) ([]uint64, error) { return nil, nil }

func (m *memComments) Comments(_ context.Context, issueID uint64) ([]domain.Comment, error) {
	var out []domain.Comment
	for id := uint64(1); id <= uint64(len(m.byID)); id++ {
		if c, ok := m.byID[id]; ok && c.IssueID == issueID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memComments) CreateConversationComment(ctx context.Context, c domain.Comment, move domain.ConversationMove) (domain.Comment, error) {
	c, _ = m.CreateComment(ctx, c)
	i := m.issues.byID[c.IssueID]
	cv := *i.Conversation
	cv.State = move.State
	if move.NewSession {
		cv.BoundaryCommentID = c.ID
	}
	i.Conversation = &cv
	m.issues.byID[i.ID] = i
	return c, nil
}

// chatService is a Guild with Members 7 and 8, Agent 3 (Ada), Agent 4
// (terminated) and the hooks a Comment calls.
func chatService(t *testing.T) (*Service, *memActivity, *[]woken) {
	t.Helper()
	issues := &memIssues{byID: map[uint64]domain.Issue{}}
	act := &memActivity{}
	s := NewService(nil, issues, &memComments{byID: map[uint64]domain.Comment{}, issues: issues}, nil, memberGuild{}, nil, act, nil, nil, nil)
	s.Logf = t.Logf
	s.Agents = func(_ context.Context, _ uint64, ids []uint64) (map[uint64]AssigneeAgent, error) {
		all := map[uint64]AssigneeAgent{3: {Name: "Ada"}, 4: {Name: "Old", Terminated: true}}
		out := map[uint64]AssigneeAgent{}
		for _, id := range ids {
			if a, ok := all[id]; ok {
				out[id] = a
			}
		}
		return out, nil
	}
	var heard []woken
	s.Commented = func(_ context.Context, i domain.Issue, c domain.Comment) error {
		heard = append(heard, woken{issue: i.ID, comment: c.ID})
		return nil
	}
	return s, act, &heard
}

func TestOpenConversation(t *testing.T) {
	ctx := context.Background()
	s, act, _ := chatService(t)
	c, err := s.OpenConversation(ctx, 1, domain.ByMember(7), 3)
	if err != nil || c.Conversation == nil || c.Title != "Chat with Ada" || c.Status != domain.InReview {
		t.Fatalf("open: %v %+v", err, c)
	}
	again, err := s.OpenConversation(ctx, 1, domain.ByMember(7), 3)
	if err != nil || again.ID != c.ID {
		t.Fatalf("open again: %v %d", err, again.ID)
	}
	if !slices.Equal(act.actions, []string{domain.IssueConversationOpenedAction}) {
		t.Errorf("activity %v", act.actions)
	}
	bo, err := s.OpenConversation(ctx, 1, domain.ByMember(8), 3)
	if err != nil || bo.ID == c.ID {
		t.Fatalf("another member: %v %d", err, bo.ID)
	}
	if _, err := s.OpenConversation(ctx, 1, domain.Actor{AgentID: 3, RunID: 9}, 3); !errors.Is(err, ErrPeopleOnly) {
		t.Errorf("agent opened one: %v", err)
	}
	for _, agent := range []uint64{4, 99} {
		var fe *domain.FieldError
		if _, err := s.OpenConversation(ctx, 1, domain.ByMember(7), agent); !errors.As(err, &fe) || fe.Field != "agent_id" {
			t.Errorf("agent %d: %v", agent, err)
		}
	}
	mine, _ := s.Conversations(ctx, 1, 7)
	if len(mine) != 1 || mine[0].ID != c.ID {
		t.Errorf("conversations %+v", mine)
	}
	if _, found, _ := s.ConversationWith(ctx, 1, 8, 3); !found {
		t.Error("bo's conversation not found")
	}
}

func TestConversationMessages(t *testing.T) {
	ctx := context.Background()
	s, _, heard := chatService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	c, _ := s.OpenConversation(ctx, 1, domain.ByMember(7), 3)
	ref := strconv.FormatUint(c.ID, 10)
	state := func() domain.Conversation {
		i, _ := s.Issue(ctx, 1, ref, all)
		return *i.Conversation
	}
	_, err := s.WriteComment(ctx, 1, domain.ByMember(7), ref, "What changed?", all)
	if err != nil || state().State != domain.ConversationActive || len(*heard) != 1 {
		t.Fatalf("owner's message: %v %+v %v", err, state(), *heard)
	}
	if _, err := s.WriteComment(ctx, 1, domain.ByMember(8), ref, "Me too", all); !errors.Is(err, domain.ErrNotConversationOwner) {
		t.Errorf("another member wrote: %v", err)
	}
	reply, err := s.WriteComment(ctx, 1, domain.Actor{AgentID: 3, RunID: 9}, ref, "Two fixes.", all)
	if err != nil || reply.RunID != 9 || state().State != domain.ConversationWaiting || len(*heard) != 1 {
		t.Fatalf("agent's reply: %v %+v %+v", err, reply, state())
	}
	fresh, err := s.WriteComment(ctx, 1, domain.ByMember(7), ref, "/new", all)
	if err != nil || state().BoundaryCommentID != fresh.ID || state().State != domain.ConversationWaiting {
		t.Fatalf("new session: %v %+v", err, state())
	}
	// The hook still hears it: agents tells a New session apart and wakes
	// nobody.
	if len(*heard) != 2 {
		t.Errorf("heard %v", *heard)
	}
	after, _ := s.WriteComment(ctx, 1, domain.ByMember(7), ref, "And now?", all)
	history, err := s.ConversationHistory(ctx, 1, c.ID, 10)
	if err != nil || len(history) != 1 || history[0].ID != after.ID {
		t.Errorf("history after the boundary: %v %+v", err, history)
	}
	if h, _ := s.ConversationHistory(ctx, 2, c.ID, 10); len(h) != 0 {
		t.Errorf("another guild's history %+v", h)
	}
	for name, p := range map[string]IssuePatch{
		"assignee": {AssigneeAgentID: ptr(uint64(0))},
		"member":   {AssigneeID: ptr(uint64(8))},
		"project":  {ProjectID: ptr(uint64(5))},
		"done":     {Status: ptr("done")},
	} {
		var fe *domain.FieldError
		if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), ref, p, all); !errors.As(err, &fe) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestConversationsAreNoWork(t *testing.T) {
	ctx := context.Background()
	s, _, _ := chatService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	c, _ := s.OpenConversation(ctx, 1, domain.ByMember(7), 3)
	work, _ := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Fix it", Status: "todo", AssigneeAgentID: 3}, all)
	is, err := s.Issues(ctx, 1, IssueFilter{}, all)
	if err != nil || len(is) != 1 || is[0].ID != work.ID {
		t.Errorf("issues %v %+v", err, is)
	}
	if is, _ := s.Issues(ctx, 1, IssueFilter{Search: "Chat"}, all); len(is) != 1 || is[0].ID != c.ID {
		t.Errorf("search %+v", is)
	}
	me := uint64(7)
	if is, _ := s.Issues(ctx, 1, IssueFilter{Search: "Chat", InboxFor: &me}, all); len(is) != 0 {
		t.Errorf("inbox search %+v", is)
	}
	if n, err := s.InboxCount(ctx, 1, 7, all); err != nil || n != 1 {
		t.Errorf("inbox count %v %d", err, n)
	}
	// The store leaves Conversations out of an Agent's open Issues; here
	// the fake does, as infra's query does.
	open, _, _ := s.AgentWork(ctx, 1, 3)
	for _, i := range open {
		if i.Conversation != nil {
			t.Errorf("agent work has %+v", i)
		}
	}
}

func TestReplyInConversation(t *testing.T) {
	ctx := context.Background()
	s, _, _ := chatService(t)
	all := func([]uint64) ([]uint64, error) { return nil, nil }
	c, _ := s.OpenConversation(ctx, 1, domain.ByMember(7), 3)
	ref := strconv.FormatUint(c.ID, 10)
	agentReplies := func() []domain.Comment {
		cs, _ := s.Comments(ctx, 1, ref, all)
		var out []domain.Comment
		for _, x := range cs {
			if x.Author.AgentID == 3 && !x.Deleted() {
				out = append(out, x)
			}
		}
		return out
	}
	s.WriteComment(ctx, 1, domain.ByMember(7), ref, "hello", all)
	if err := s.ReplyInConversation(ctx, 1, c.ID, 3, 9, "Hi."); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplyInConversation(ctx, 1, c.ID, 3, 9, "Hi again."); err != nil {
		t.Fatal(err)
	}
	got := agentReplies()
	if len(got) != 1 || got[0].Body != "Hi." || got[0].RunID != 9 {
		t.Fatalf("replies %+v", got)
	}
	if i, _ := s.Issue(ctx, 1, ref, all); i.Conversation.State != domain.ConversationWaiting {
		t.Errorf("state %s", i.Conversation.State)
	}
	// The Agent already wrote in Run 10 through the API: no Completion
	// reply.
	s.WriteComment(ctx, 1, domain.Actor{AgentID: 3, RunID: 10}, ref, "Answered myself.", all)
	s.ReplyInConversation(ctx, 1, c.ID, 3, 10, "Answered myself.")
	// Another Guild's, or another Agent's, writes nothing; a long text is
	// cut.
	s.ReplyInConversation(ctx, 2, c.ID, 3, 11, "Not here.")
	s.ReplyInConversation(ctx, 1, c.ID, 4, 11, "Not mine.")
	s.ReplyInConversation(ctx, 1, c.ID, 3, 12, strings.Repeat("x", domain.MaxComment+5))
	got = agentReplies()
	if len(got) != 3 || len(got[2].Body) != domain.MaxComment {
		t.Errorf("replies %d", len(got))
	}
}
