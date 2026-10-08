package app

import (
	"context"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// Me is what an Agent asking through its Run key is told about itself:
// the Agent, its Guild, the Run the key belongs to, with its Issue when
// the Agent may view it, and its Chain of command.
type Me struct {
	Agent       domain.Agent
	GuildName   string
	IssuePrefix string
	Run         domain.Run
	Issue       *IssueBrief
	Chain       []domain.Agent
}

// Me tells the Agent about itself during its Run.
func (s *Service) Me(ctx context.Context, guildID, agentID, runID uint64, visible Visible) (Me, error) {
	a, err := s.Agent(ctx, guildID, agentID)
	if err != nil {
		return Me{}, err
	}
	r, err := s.Run(ctx, guildID, runID)
	if err != nil {
		return Me{}, err
	}
	all, err := s.agents.Agents(ctx, guildID)
	if err != nil {
		return Me{}, err
	}
	m := Me{Agent: a, Run: r, Chain: domain.ChainOfCommand(all, a)}
	if m.GuildName, err = s.guilds.GuildName(ctx, guildID); err != nil {
		return Me{}, err
	}
	if m.IssuePrefix, err = s.guilds.IssuePrefix(ctx, guildID); err != nil {
		return Me{}, err
	}
	if r.IssueID != 0 {
		i, ok, err := s.issueOf(ctx, guildID, r.IssueID, visible)
		if err != nil {
			return Me{}, err
		}
		if ok {
			m.Issue = &i
		}
	}
	return m, nil
}

// InboxIssue is an Issue in an Agent's inbox, from work.
type InboxIssue struct {
	ID            uint64
	Identifier    string
	Title         string
	Status        string
	Priority      string
	ProjectID     uint64
	ProjectName   string
	GoalID        uint64
	ParentID      uint64
	CheckoutRunID uint64
	UpdatedAt     time.Time
}

// Inbox lists the Issues assigned to the Agent that it can work on, as
// work orders them, leaving out those in Projects it may not view.
func (s *Service) Inbox(ctx context.Context, guildID, agentID uint64, visible Visible) ([]InboxIssue, error) {
	is, err := s.work.InboxOfAgent(ctx, guildID, agentID)
	if err != nil || len(is) == 0 {
		return nil, err
	}
	var projectIDs []uint64
	for _, i := range is {
		if i.ProjectID != 0 {
			projectIDs = append(projectIDs, i.ProjectID)
		}
	}
	seen := map[uint64]bool{}
	if len(projectIDs) > 0 {
		ids, err := visible(projectIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			seen[id] = true
		}
	}
	out := make([]InboxIssue, 0, len(is))
	for _, i := range is {
		if i.ProjectID == 0 || seen[i.ProjectID] {
			out = append(out, i)
		}
	}
	return out, nil
}
