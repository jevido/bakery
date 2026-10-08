package app

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// CostRange picks the Guild's Runs Costs adds up: those claimed (started_at
// set) that finished in [From, To). A nil bound is open.
type CostRange struct {
	GuildID  uint64
	From, To *time.Time
}

// Figures are what a set of Runs used, added up from their Run usage.
type Figures struct {
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	Runs              int64
	// RunTimeMS is the sum of the Runs' Run usage duration.
	RunTimeMS         int64
	CostEquivalentUSD float64
}

// Tokens is what the tokens Budget metric counts: input plus output;
// cached input tokens are cache reads and are not counted.
func (f Figures) Tokens() int64 {
	return f.InputTokens + f.OutputTokens
}

func (f *Figures) add(o Figures) {
	f.InputTokens += o.InputTokens
	f.CachedInputTokens += o.CachedInputTokens
	f.OutputTokens += o.OutputTokens
	f.Runs += o.Runs
	f.RunTimeMS += o.RunTimeMS
	f.CostEquivalentUSD += o.CostEquivalentUSD
}

// RunTotal is the Figures of the Runs of one Agent for one Project (0 for
// none), as the Runs port adds them up.
type RunTotal struct {
	AgentID   uint64
	ProjectID uint64
	Figures
}

// CostRuns is what Costs asks the Runs port.
type CostRuns interface {
	// RunTotals adds up the Runs in the range per Agent and Project.
	RunTotals(ctx context.Context, q CostRange) ([]RunTotal, error)
}

// AgentCost is one Agent's Figures in Costs.
type AgentCost struct {
	Agent domain.Agent
	Figures
}

// ProjectCost is one Project's Figures in Costs; ProjectID 0 is the Runs
// without a Project.
type ProjectCost struct {
	ProjectID   uint64
	ProjectName string
	Figures
}

// Costs is what the Guild's Runs in a range used: in total, per Agent and
// per Project, each list by tokens, most first.
type Costs struct {
	Total    Figures
	Agents   []AgentCost
	Projects []ProjectCost
}

// Costs adds up the Guild's Runs in the range. Projects the person asking
// may not view (visible) count in the total and their Agents' rows but get
// no row of their own.
func (s *Service) Costs(ctx context.Context, q CostRange, visible Visible) (Costs, error) {
	totals, err := s.runs.RunTotals(ctx, q)
	if err != nil {
		return Costs{}, err
	}
	var out Costs
	byAgent := map[uint64]*Figures{}
	byProject := map[uint64]*Figures{}
	for _, t := range totals {
		out.Total.add(t.Figures)
		for _, m := range []struct {
			into map[uint64]*Figures
			id   uint64
		}{{byAgent, t.AgentID}, {byProject, t.ProjectID}} {
			if m.into[m.id] == nil {
				m.into[m.id] = &Figures{}
			}
			m.into[m.id].add(t.Figures)
		}
	}
	if len(byAgent) > 0 {
		all, err := s.agents.Agents(ctx, q.GuildID)
		if err != nil {
			return Costs{}, err
		}
		for _, a := range all {
			if f, ok := byAgent[a.ID]; ok {
				out.Agents = append(out.Agents, AgentCost{Agent: a, Figures: *f})
			}
		}
	}
	ids := make([]uint64, 0, len(byProject))
	for id := range byProject {
		if id != 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 && visible != nil {
		if ids, err = visible(ids); err != nil {
			return Costs{}, err
		}
	}
	if len(ids) > 0 {
		names, err := s.repositories.ProjectNames(ctx, q.GuildID, ids)
		if err != nil {
			return Costs{}, err
		}
		for _, id := range ids {
			if name, ok := names[id]; ok {
				out.Projects = append(out.Projects, ProjectCost{ProjectID: id, ProjectName: name, Figures: *byProject[id]})
			}
		}
	}
	if f, ok := byProject[0]; ok {
		out.Projects = append(out.Projects, ProjectCost{Figures: *f})
	}
	slices.SortStableFunc(out.Agents, func(x, y AgentCost) int {
		return cmp.Or(cmp.Compare(y.Tokens(), x.Tokens()), strings.Compare(strings.ToLower(x.Agent.Name), strings.ToLower(y.Agent.Name)))
	})
	slices.SortStableFunc(out.Projects, func(x, y ProjectCost) int {
		return cmp.Or(cmp.Compare(y.Tokens(), x.Tokens()), strings.Compare(strings.ToLower(x.ProjectName), strings.ToLower(y.ProjectName)))
	})
	return out, nil
}
