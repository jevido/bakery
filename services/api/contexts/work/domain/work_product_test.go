package domain

import (
	"errors"
	"testing"
)

func TestWorkProductMove(t *testing.T) {
	cases := []struct {
		typ      WorkProductType
		from, to WorkProductStatus
		ok       bool
	}{
		{PullRequestProduct, PullRequestOpen, PullRequestMerged, true},
		{PullRequestProduct, PullRequestOpen, PullRequestClosed, true},
		{PullRequestProduct, PullRequestClosed, PullRequestOpen, true},
		{PullRequestProduct, PullRequestMerged, PullRequestOpen, false},
		{PullRequestProduct, PullRequestMerged, PullRequestClosed, false},
		{PullRequestProduct, PullRequestClosed, PullRequestMerged, false},
		{PullRequestProduct, PullRequestOpen, PreviewReady, false},
		{PreviewURLProduct, PreviewDeploying, PreviewReady, true},
		{PreviewURLProduct, PreviewReady, PreviewDeploying, true},
		{PreviewURLProduct, PreviewFailed, PreviewRemoved, true},
		{PreviewURLProduct, PreviewRemoved, PreviewDeploying, true},
		{PreviewURLProduct, PreviewRemoved, PreviewReady, false},
		{PreviewURLProduct, PreviewReady, PullRequestOpen, false},
	}
	for _, c := range cases {
		w := WorkProduct{Type: c.typ, Status: c.from}
		changed, err := w.Move(c.to)
		if c.ok != (err == nil) || changed != c.ok || (c.ok && w.Status != c.to) {
			t.Errorf("%s %s → %s: changed %v, %v", c.typ, c.from, c.to, changed, err)
		}
		if !c.ok && !errors.Is(err, ErrWorkProductMove) {
			t.Errorf("%s %s → %s: %v", c.typ, c.from, c.to, err)
		}
	}
	w := WorkProduct{Type: PullRequestProduct, Status: PullRequestOpen}
	if changed, err := w.Move(PullRequestOpen); changed || err != nil {
		t.Errorf("same status: %v %v", changed, err)
	}
	w = NewPullRequest(Issue{ID: 3, GuildID: 2}, 7, "forgejo", 12, "DEF-1 Fix", "http://git/pr/12", ByAgent(5))
	if w.ExternalID != "12" || w.Status != PullRequestOpen || w.IssueID != 3 || w.GuildID != 2 || w.CreatedBy.AgentID != 5 {
		t.Errorf("new: %+v", w)
	}
}
