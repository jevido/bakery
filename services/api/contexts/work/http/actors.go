package http

import (
	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// Agent is an Agent as the author or Actor of something shows it.
type Agent struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// actor is who the request acts as: its Agent for a Run key, else its
// Member.
func (c *Controller) actor(ctx contractshttp.Context) domain.Actor {
	if c.Agent != nil {
		if id := c.Agent(ctx); id != 0 {
			return domain.ByAgent(id)
		}
	}
	return domain.ByMember(c.Member(ctx))
}

// actorNames is what the Actors of a response are called: its Members and
// its Agents, terminated ones too. A missing one is gone.
type actorNames struct {
	members map[uint64]Member
	agents  map[uint64]app.AssigneeAgent
}

// actorNames names the Actors in one call for the Members and one for the
// Agents.
func (c *Controller) actorNames(ctx contractshttp.Context, actors []domain.Actor) (actorNames, error) {
	var memberIDs, agentIDs []uint64
	for _, a := range actors {
		if a.MemberID != 0 {
			memberIDs = append(memberIDs, a.MemberID)
		}
		if a.AgentID != 0 {
			agentIDs = append(agentIDs, a.AgentID)
		}
	}
	ms, err := c.memberMap(ctx.Context(), memberIDs)
	if err != nil {
		return actorNames{}, err
	}
	as := map[uint64]app.AssigneeAgent{}
	if len(agentIDs) > 0 {
		if as, err = c.service.AssigneeAgents(ctx.Context(), c.guild(ctx), agentIDs); err != nil {
			return actorNames{}, err
		}
	}
	return actorNames{members: ms, agents: as}, nil
}

// member is the Actor when it is a Member, nil otherwise.
func (n actorNames) member(a domain.Actor) *Member {
	return memberOf(n.members, a.MemberID)
}

// agent is the Actor when it is an Agent, nil otherwise.
func (n actorNames) agent(a domain.Actor) *Agent {
	if g, ok := n.agents[a.AgentID]; ok && a.AgentID != 0 {
		return &Agent{ID: a.AgentID, Name: g.Name, Icon: g.Icon}
	}
	return nil
}
