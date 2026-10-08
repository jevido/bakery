package app

import (
	"context"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

func TestFollowPullRequestAndPreview(t *testing.T) {
	ctx := context.Background()
	act := &memActivity{}
	s := NewService(nil, &memIssues{byID: map[uint64]domain.Issue{}}, nil, nil, memberGuild{}, memProjects{}, act, nil, nil)
	s.Logf = t.Logf
	products := &memWorkProducts{}
	s.WorkProducts = products
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	for _, title := range []string{"Fix it", "Other"} {
		if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: title, Status: "todo", ProjectID: 1}, all); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), "1", IssuePatch{ApplicationID: ptr(uint64(10))}, all); err != nil {
		t.Fatal(err)
	}
	follow := func(e PullRequestEvent) {
		t.Helper()
		e.GuildID, e.ApplicationID, e.Provider = max(e.GuildID, 1), 10, "forgejo"
		if err := s.FollowPullRequest(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	preview := func(e PreviewEvent) {
		t.Helper()
		e.ApplicationID = 10
		if err := s.FollowPreview(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	status := func(typ domain.WorkProductType, n string) domain.WorkProductStatus {
		w, _, _ := products.WorkProduct(ctx, 1, typ, n)
		return w.Status
	}

	// Not an Agent branch, another Guild, another Issue's branch: nothing.
	follow(PullRequestEvent{Number: 3, Action: "opened", Branch: "feature"})
	follow(PullRequestEvent{GuildID: 2, Number: 3, Action: "opened", Branch: "bakery/bak-1"})
	follow(PullRequestEvent{Number: 3, Action: "opened", Branch: "bakery/bak-2"})
	if len(products.all) != 0 {
		t.Fatalf("recorded %+v", products.all)
	}
	// One opened by hand from the Issue's Agent branch is recorded.
	follow(PullRequestEvent{Number: 3, Action: "opened", Branch: "bakery/bak-1", Title: "By hand", URL: "http://git/pulls/3"})
	w := products.all[0]
	if w.IssueID != 1 || w.Status != domain.PullRequestOpen || !w.CreatedBy.None() || w.Title != "By hand" || w.Provider != "forgejo" {
		t.Fatalf("by hand: %+v", w)
	}
	if act.actions[len(act.actions)-1] != domain.PullRequestOpenedAction {
		t.Fatalf("activity %v", act.actions)
	}

	// Its Preview: deploying without a link, then ready with it.
	preview(PreviewEvent{Number: 99, Status: domain.PreviewDeploying})
	preview(PreviewEvent{Number: 3, Status: domain.PreviewDeploying})
	if len(products.all) != 2 || status(domain.PreviewURLProduct, "3") != domain.PreviewDeploying {
		t.Fatalf("deploying: %+v", products.all)
	}
	preview(PreviewEvent{GuildID: 1, Number: 3, Status: domain.PreviewReady, URL: "https://pr-3.app.test"})
	if p, _, _ := products.WorkProduct(ctx, 1, domain.PreviewURLProduct, "3"); p.Status != domain.PreviewReady || p.URL != "https://pr-3.app.test" {
		t.Fatalf("ready: %+v", p)
	}
	if !slices.Contains(act.actions, domain.PreviewReadyAction) {
		t.Fatalf("activity %v", act.actions)
	}
	// Another Guild's event is not this Issue's.
	preview(PreviewEvent{GuildID: 2, Number: 3, Status: domain.PreviewFailed})
	if status(domain.PreviewURLProduct, "3") != domain.PreviewReady {
		t.Fatal("another guild's preview moved it")
	}

	// A push touches it only; closing closes it and removes the Preview.
	n := len(act.actions)
	follow(PullRequestEvent{Number: 3, Action: "pushed", Branch: "bakery/bak-1"})
	if status(domain.PullRequestProduct, "3") != domain.PullRequestOpen || len(act.actions) != n {
		t.Fatalf("pushed: %v", act.actions)
	}
	follow(PullRequestEvent{Number: 3, Action: "closed", Branch: "bakery/bak-1"})
	preview(PreviewEvent{Number: 3, Status: domain.PreviewRemoved})
	if status(domain.PullRequestProduct, "3") != domain.PullRequestClosed || status(domain.PreviewURLProduct, "3") != domain.PreviewRemoved {
		t.Fatalf("closed: %+v", products.all)
	}
	if act.actions[len(act.actions)-1] != domain.PullRequestClosedAction {
		t.Fatalf("activity %v", act.actions)
	}
	// Reopened, its Preview deploys again and fails.
	follow(PullRequestEvent{Number: 3, Action: "opened", Branch: "bakery/bak-1"})
	preview(PreviewEvent{Number: 3, Status: domain.PreviewDeploying})
	preview(PreviewEvent{Number: 3, Status: domain.PreviewFailed})
	if status(domain.PullRequestProduct, "3") != domain.PullRequestOpen || status(domain.PreviewURLProduct, "3") != domain.PreviewFailed {
		t.Fatalf("reopened: %+v", products.all)
	}
	if act.actions[len(act.actions)-1] != domain.PreviewFailedAction {
		t.Fatalf("activity %v", act.actions)
	}
	// Merged, it stays merged whatever comes late.
	follow(PullRequestEvent{Number: 3, Action: "closed", Merged: true, Branch: "bakery/bak-1"})
	follow(PullRequestEvent{Number: 3, Action: "opened", Branch: "bakery/bak-1"})
	if status(domain.PullRequestProduct, "3") != domain.PullRequestMerged || !slices.Contains(act.actions, domain.PullRequestMergedAction) {
		t.Fatalf("merged: %+v %v", products.all, act.actions)
	}
	// A Preview removed that the Issue never saw records nothing.
	preview(PreviewEvent{Number: 4, Status: domain.PreviewRemoved})
	if len(products.all) != 2 {
		t.Fatalf("products %+v", products.all)
	}
}
