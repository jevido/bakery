package app

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// ErrPeopleOnly is a Conversation opened by an Agent: only a Member owns
// one.
var ErrPeopleOnly = errors.New("only a person can open a conversation")

// OpenConversation answers the Member's Conversation with the Guild's
// Agent, creating it under the Guild's Issue counter the first time. The
// Agent must be the Guild's and not terminated. Two requests at once
// answer the same Conversation: the one that loses the unique index
// reads the row the other created.
func (s *Service) OpenConversation(ctx context.Context, guildID uint64, by domain.Actor, agentID uint64) (domain.Issue, error) {
	if by.AgentID != 0 || by.MemberID == 0 {
		return domain.Issue{}, ErrPeopleOnly
	}
	if i, found, err := s.issues.ConversationOf(ctx, guildID, by.MemberID, agentID); err != nil || found {
		return i, err
	}
	agents, err := s.AssigneeAgents(ctx, guildID, []uint64{agentID})
	if err != nil {
		return domain.Issue{}, err
	}
	a, ok := agents[agentID]
	if !ok || a.Terminated {
		return domain.Issue{}, &domain.FieldError{Field: "agent_id", Message: "agent must be an agent of this guild that is not terminated"}
	}
	i, err := domain.NewConversation(guildID, agentID, by.MemberID, a.Name)
	if err != nil {
		return domain.Issue{}, err
	}
	created, err := s.issues.CreateIssue(ctx, i)
	if err != nil {
		if again, found, ferr := s.issues.ConversationOf(ctx, guildID, by.MemberID, agentID); ferr == nil && found {
			return again, nil
		}
		return domain.Issue{}, err
	}
	s.publish(ctx, domain.ConversationOpened{Happened: s.happened(by), Issue: created, AgentName: a.Name})
	return created, nil
}

// Conversations lists the Member's Conversations in the Guild, most
// recently updated first.
func (s *Service) Conversations(ctx context.Context, guildID, memberID uint64) ([]domain.Issue, error) {
	return s.issues.Conversations(ctx, guildID, memberID)
}

// ConversationWith finds the Member's Conversation with the Agent; found
// is false when they have none.
func (s *Service) ConversationWith(ctx context.Context, guildID, memberID, agentID uint64) (domain.Issue, bool, error) {
	return s.issues.ConversationOf(ctx, guildID, memberID, agentID)
}

// ConversationHistory is the newest limit Comments of the Guild's
// Conversation after its Session boundary, deleted ones left out, oldest
// first: what a Conversation Run's prompt quotes. Another Guild's Issue,
// or one that is no Conversation, has none.
func (s *Service) ConversationHistory(ctx context.Context, guildID, issueID uint64, limit int) ([]domain.Comment, error) {
	i, found, err := s.issues.Issue(ctx, issueID)
	if err != nil || !found || i.GuildID != guildID || i.Conversation == nil {
		return nil, err
	}
	all, err := s.comments.Comments(ctx, i.ID)
	if err != nil {
		return nil, err
	}
	var out []domain.Comment
	for _, c := range all {
		if c.ID == i.Conversation.BoundaryCommentID {
			out = out[:0]
			continue
		}
		if !c.Deleted() {
			out = append(out, c)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// ReplyInConversation writes the Completion reply: the body as the
// Conversation agent's Comment in its Run, through WriteComment, cut to
// domain.MaxComment characters. It does
// nothing when the Agent already wrote a Comment in that Run that is not
// deleted, so a Run answers once, and nothing for an Issue that is no
// Conversation of that Agent.
func (s *Service) ReplyInConversation(ctx context.Context, guildID, issueID, agentID, runID uint64, body string) error {
	i, found, err := s.issues.Issue(ctx, issueID)
	if err != nil || !found || i.GuildID != guildID || i.Conversation == nil || i.Conversation.AgentID != agentID {
		return err
	}
	cs, err := s.comments.Comments(ctx, i.ID)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.Author.AgentID == agentID && c.RunID == runID && !c.Deleted() {
			return nil
		}
	}
	if r := []rune(strings.TrimSpace(body)); len(r) > domain.MaxComment {
		body = string(r[:domain.MaxComment])
	}
	_, err = s.WriteComment(ctx, guildID, domain.Actor{AgentID: agentID, RunID: runID}, strconv.FormatUint(i.ID, 10), body, nil)
	return err
}
