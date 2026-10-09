package domain

import (
	"errors"
	"strings"
)

// ConversationState is whose turn it is in a Conversation: active while
// the Agent has a message of its owner to answer, waiting once it
// answered.
type ConversationState string

const (
	ConversationActive  ConversationState = "active"
	ConversationWaiting ConversationState = "waiting"
)

// Conversation is what makes an Issue a Conversation: one Member's (its
// Conversation owner's) chat with one Agent of the Guild (its
// Conversation agent). BoundaryCommentID is its Session boundary, 0 for
// none: the Agent sees only the Comments after it.
type Conversation struct {
	AgentID           uint64
	MemberID          uint64
	State             ConversationState
	BoundaryCommentID uint64
}

// NewSessionCommand is the body of a Comment that starts a New session.
const NewSessionCommand = "/new"

var (
	// ErrNotConversationOwner is a Comment in a Conversation by anyone but
	// its owner or its Agent.
	ErrNotConversationOwner = errors.New("only the conversation's owner can write in it")
	// ErrConversationClosed is a Conversation moved to done or cancelled.
	ErrConversationClosed error = &FieldError{Field: "status", Message: "a conversation cannot be done or cancelled"}
)

func fixedForConversation(field string) error {
	return &FieldError{Field: field, Message: "is fixed for a conversation"}
}

// NewConversation is the Member's Conversation with the Agent of the
// Guild, named agentName: an Issue titled "Chat with <name>" (cut to the
// longest title), in review, assigned to the Agent and waiting for the
// owner's first message. Its number is given when it is stored.
func NewConversation(guildID, agentID, memberID uint64, agentName string) (Issue, error) {
	title := []rune("Chat with " + strings.TrimSpace(agentName))
	if len(title) > MaxTitle {
		title = title[:MaxTitle]
	}
	i, err := NewIssue(guildID, ByMember(memberID), string(title), "")
	if err != nil {
		return Issue{}, err
	}
	i.Status, i.AssigneeAgentID = InReview, agentID
	i.Conversation = &Conversation{AgentID: agentID, MemberID: memberID, State: ConversationWaiting}
	return i, nil
}

// IsNewSession tells whether a Comment's body starts a New session.
func IsNewSession(body string) bool { return strings.TrimSpace(body) == NewSessionCommand }

// ConversationMove is what a Comment does to its Conversation: the
// Conversation state after it, and whether it is the new Session
// boundary.
type ConversationMove struct {
	State      ConversationState
	NewSession bool
}

// Converse is what a Comment by the actor with the body does to the
// Conversation: its owner's message makes it active, its owner's "/new"
// starts a New session and leaves the state, and its Agent's message
// makes it waiting. Anyone else is ErrNotConversationOwner.
func (i Issue) Converse(by Actor, body string) (ConversationMove, error) {
	c := i.Conversation
	switch {
	case c == nil:
		return ConversationMove{}, errors.New("issue is not a conversation")
	case by.AgentID == 0 && by.MemberID != 0 && by.MemberID == c.MemberID:
		if IsNewSession(body) {
			return ConversationMove{State: c.State, NewSession: true}, nil
		}
		return ConversationMove{State: ConversationActive}, nil
	case by.AgentID != 0 && by.AgentID == c.AgentID:
		return ConversationMove{State: ConversationWaiting}, nil
	}
	return ConversationMove{}, ErrNotConversationOwner
}
