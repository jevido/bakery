package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewConversation(t *testing.T) {
	i, err := NewConversation(1, 3, 7, "Release Bot")
	if err != nil {
		t.Fatal(err)
	}
	if i.Title != "Chat with Release Bot" || i.Status != InReview || i.AssigneeAgentID != 3 || i.CreatedBy != ByMember(7) ||
		*i.Conversation != (Conversation{AgentID: 3, MemberID: 7, State: ConversationWaiting}) {
		t.Fatalf("conversation %+v %+v", i, i.Conversation)
	}
	long, err := NewConversation(1, 3, 7, strings.Repeat("a", 300))
	if err != nil || len([]rune(long.Title)) != MaxTitle {
		t.Errorf("long name: %v %d", err, len(long.Title))
	}
}

func TestConversationIsFixed(t *testing.T) {
	now := time.Now()
	fixed := func(field string, err error) {
		t.Helper()
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
	i, _ := NewConversation(1, 3, 7, "Ada")
	i.ID = 10
	fixed("status", i.SetStatus(Done, now))
	fixed("status", i.SetStatus(IssueCancelled, now))
	if err := i.SetStatus(InProgress, now); err != nil {
		t.Errorf("in progress: %v", err)
	}
	fixed("assignee_id", i.Assign(8))
	fixed("assignee_agent_id", i.AssignAgent(4))
	fixed("assignee_agent_id", i.AssignAgent(0))
	if err := i.AssignAgent(3); err != nil || i.Assign(0) != nil {
		t.Errorf("its own agent: %v", err)
	}
	fixed("project_id", i.PlaceIn(5))
	fixed("application_id", i.SetApplication(6))
	fixed("goal_id", i.ServeGoal(&Goal{ID: 2, GuildID: 1}))
	other := Issue{ID: 11, GuildID: 1}
	fixed("parent_id", i.MoveUnder(&other, nil))
	fixed("parent_id", other.MoveUnder(&i, nil))
	_, err := i.BlockWith([]Issue{other}, nil)
	fixed("blocked_by_ids", err)
	_, err = other.BlockWith([]Issue{i}, nil)
	fixed("blocked_by_ids", err)
	if i.AssigneeAgentID != 3 || i.ProjectID != 0 || i.ParentID != 0 || other.ParentID != 0 {
		t.Errorf("changed anyway: %+v", i)
	}
	// Release lets go of the Checkout, never of the Conversation agent.
	if _, err := i.Checkout(3, 20, []IssueStatus{InProgress}, func(uint64) bool { return false }, now); err != nil {
		t.Fatal(err)
	}
	if err := i.Release(20, now); err != nil || i.AssigneeAgentID != 3 {
		t.Errorf("release: %v %d", err, i.AssigneeAgentID)
	}
}

func TestConverse(t *testing.T) {
	i, _ := NewConversation(1, 3, 7, "Ada")
	for _, c := range []struct {
		by   Actor
		body string
		want ConversationMove
		err  error
	}{
		{ByMember(7), "What changed?", ConversationMove{State: ConversationActive}, nil},
		{ByMember(7), "  /new\n", ConversationMove{State: ConversationWaiting, NewSession: true}, nil},
		{ByMember(7), "/new please", ConversationMove{State: ConversationActive}, nil},
		{Actor{AgentID: 3, RunID: 9}, "Here it is.", ConversationMove{State: ConversationWaiting}, nil},
		{ByAgent(3), "/new", ConversationMove{State: ConversationWaiting}, nil},
		{ByMember(8), "Me too", ConversationMove{}, ErrNotConversationOwner},
		{ByAgent(4), "Me too", ConversationMove{}, ErrNotConversationOwner},
	} {
		got, err := i.Converse(c.by, c.body, 4)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%+v %q: %+v %v", c.by, c.body, got, err)
		}
	}
}

func TestNewBoardChat(t *testing.T) {
	i, err := NewBoardChat(1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if i.Title != "Board Operations" || i.Status != InReview || i.AssigneeAgentID != 0 || i.AssigneeID != 0 ||
		i.CreatedBy != ByMember(7) || !i.IsBoardChat() ||
		*i.Conversation != (Conversation{Board: true, State: ConversationWaiting}) {
		t.Fatalf("board chat %+v %+v", i, i.Conversation)
	}
	if c, _ := NewConversation(1, 3, 7, "Ada"); c.IsBoardChat() || (Issue{}).IsBoardChat() {
		t.Error("only the board chat is one")
	}
}

func TestBoardChatIsFixed(t *testing.T) {
	now := time.Now()
	fixed := func(field string, err error) {
		t.Helper()
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
	i, _ := NewBoardChat(1, 7)
	i.ID = 10
	fixed("status", i.SetStatus(Done, now))
	fixed("status", i.SetStatus(IssueCancelled, now))
	fixed("assignee_id", i.Assign(8))
	fixed("assignee_agent_id", i.AssignAgent(4))
	if err := i.AssignAgent(0); err != nil || i.Assign(0) != nil {
		t.Errorf("no assignee: %v", err)
	}
	fixed("project_id", i.PlaceIn(5))
	fixed("goal_id", i.ServeGoal(&Goal{ID: 2, GuildID: 1}))
	other := Issue{ID: 11, GuildID: 1}
	fixed("parent_id", i.MoveUnder(&other, nil))
	fixed("parent_id", other.MoveUnder(&i, nil))
	_, err := i.BlockWith([]Issue{other}, nil)
	fixed("blocked_by_ids", err)
	// Nobody is assigned it, so no Agent can take it by a Checkout.
	if _, err := i.Checkout(4, 20, []IssueStatus{InReview}, func(uint64) bool { return false }, now); !errors.Is(err, ErrNotAssignee) {
		t.Errorf("checkout: %v", err)
	}
	if i.AssigneeAgentID != 0 || i.ProjectID != 0 || i.ParentID != 0 || i.Status != InReview {
		t.Errorf("changed anyway: %+v", i)
	}
}

func TestConverseInBoardChat(t *testing.T) {
	i, _ := NewBoardChat(1, 7)
	for _, c := range []struct {
		by   Actor
		body string
		ceo  uint64
		want ConversationMove
		err  error
	}{
		{ByMember(7), "Plan the quarter.", 3, ConversationMove{State: ConversationActive}, nil},
		{ByMember(8), "And hiring.", 3, ConversationMove{State: ConversationActive}, nil},
		{ByMember(8), "/new", 3, ConversationMove{State: ConversationWaiting, NewSession: true}, nil},
		{ByMember(9), "No CEO yet.", 0, ConversationMove{State: ConversationActive}, nil},
		{Actor{AgentID: 3, RunID: 9}, "Here is the plan.", 3, ConversationMove{State: ConversationWaiting}, nil},
		{ByAgent(4), "Me too", 3, ConversationMove{}, ErrNotConversationOwner},
		{ByAgent(3), "Not the CEO now", 0, ConversationMove{}, ErrNotConversationOwner},
	} {
		got, err := i.Converse(c.by, c.body, c.ceo)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%+v %q: %+v %v", c.by, c.body, got, err)
		}
	}
}

func TestCommentKeepsAgentsRun(t *testing.T) {
	c, _ := NewComment(1, Actor{AgentID: 3, RunID: 9}, "Done.")
	m, _ := NewComment(1, Actor{MemberID: 7, RunID: 9}, "Thanks.")
	if c.RunID != 9 || c.Author != ByAgent(3) || m.RunID != 0 {
		t.Errorf("run ids %+v %+v", c, m)
	}
}
