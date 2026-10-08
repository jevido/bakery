package app

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/agents/domain"
)

// RunTotals adds up the claimed Runs that finished in the range, as the
// GROUP BY does.
func (f *fakeRuns) RunTotals(_ context.Context, q CostRange) ([]RunTotal, error) {
	var out []RunTotal
	for id := uint64(1); id <= f.next; id++ {
		r, ok := f.rows[id]
		if !ok || r.GuildID != q.GuildID || r.StartedAt == nil || r.FinishedAt == nil {
			continue
		}
		if (q.From != nil && r.FinishedAt.Before(*q.From)) || (q.To != nil && !r.FinishedAt.Before(*q.To)) {
			continue
		}
		u := r.Usage
		f := Figures{InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, OutputTokens: u.OutputTokens,
			Runs: 1, RunTimeMS: u.DurationMS, CostEquivalentUSD: u.CostEquivalentUSD}
		i := slices.IndexFunc(out, func(t RunTotal) bool { return t.AgentID == r.AgentID && t.ProjectID == r.ProjectID })
		if i < 0 {
			out = append(out, RunTotal{AgentID: r.AgentID, ProjectID: r.ProjectID})
			i = len(out) - 1
		}
		out[i].add(f)
	}
	return out, nil
}

func TestClaimRemembersTheProject(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	_, r := queuedRun(t, s, w)
	// The Issue moved to Project 12 after the Run was queued: the claim
	// takes the Project it is in now.
	i := w.issues[30]
	i.ProjectID = 12
	w.issues[30] = i
	q, err := s.ClaimRun(ctx, Desktop{ID: 3, MemberID: 7}, r.ID)
	if err != nil || q.Run.ProjectID != 12 {
		t.Fatalf("claim: %+v %v", q.Run, err)
	}
	if got := s.runs.(*fakeRuns).rows[r.ID].ProjectID; got != 12 {
		t.Errorf("stored project %d", got)
	}
}

func TestCostsAddUpRuns(t *testing.T) {
	ctx := context.Background()
	s, _, _, w := newTest()
	ada, bob := hired(t, s, "Ada", 0), hired(t, s, "Bob", 0)
	w.projects = map[uint64]string{12: "Shop", 13: "Secret"}
	runs := s.runs.(*fakeRuns)
	day := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	add := func(agentID, projectID uint64, in, out int64, at time.Time, claimed bool) {
		runs.next++
		r := domain.Run{ID: runs.next, GuildID: 1, AgentID: agentID, ProjectID: projectID, Status: domain.RunSucceeded, FinishedAt: &at,
			Usage: domain.Usage{InputTokens: in, CachedInputTokens: 5, OutputTokens: out, DurationMS: 1000, CostEquivalentUSD: 0.5}}
		if claimed {
			r.StartedAt = &at
		}
		runs.rows[r.ID] = r
	}
	add(ada.ID, 12, 100, 10, day, true)
	add(ada.ID, 12, 100, 10, day.Add(time.Hour), true)
	add(ada.ID, 0, 50, 5, day, true)
	add(bob.ID, 13, 1000, 100, day, true)
	add(bob.ID, 12, 7, 7, day, false)                  // cancelled before a claim: not counted
	add(bob.ID, 12, 1, 1, day.AddDate(0, -1, 0), true) // before the range

	from, to := day.Add(-time.Hour), day.Add(2*time.Hour)
	hide13 := func(ids []uint64) ([]uint64, error) {
		return slices.DeleteFunc(slices.Clone(ids), func(id uint64) bool { return id == 13 }), nil
	}
	c, err := s.Costs(ctx, CostRange{GuildID: 1, From: &from, To: &to}, hide13)
	if err != nil {
		t.Fatal(err)
	}
	if c.Total.Runs != 4 || c.Total.Tokens() != 1375 || c.Total.CachedInputTokens != 20 || c.Total.RunTimeMS != 4000 || c.Total.CostEquivalentUSD != 2 {
		t.Errorf("total %+v", c.Total)
	}
	if len(c.Agents) != 2 || c.Agents[0].Agent.ID != bob.ID || c.Agents[0].Tokens() != 1100 || c.Agents[1].Runs != 3 {
		t.Errorf("by agent %+v", c.Agents)
	}
	// Project 13 is hidden from the asker: in the total, without a row.
	if len(c.Projects) != 2 || c.Projects[0].ProjectName != "Shop" || c.Projects[0].Runs != 2 ||
		c.Projects[1].ProjectID != 0 || c.Projects[1].Runs != 1 {
		t.Errorf("by project %+v", c.Projects)
	}
	// A range after every Run has nothing.
	later := to.AddDate(1, 0, 0)
	if c, _ := s.Costs(ctx, CostRange{GuildID: 1, From: &later}, nil); c.Total != (Figures{}) || len(c.Agents)+len(c.Projects) != 0 {
		t.Errorf("empty range %+v", c)
	}
}
