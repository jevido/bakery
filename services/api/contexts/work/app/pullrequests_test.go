package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type memWorkProducts struct{ all []domain.WorkProduct }

func (m *memWorkProducts) WorkProducts(_ context.Context, issueID uint64) ([]domain.WorkProduct, error) {
	var out []domain.WorkProduct
	for _, w := range slices.Backward(m.all) {
		if w.IssueID == issueID {
			out = append(out, w)
		}
	}
	return out, nil
}

func (m *memWorkProducts) WorkProduct(_ context.Context, issueID uint64, typ domain.WorkProductType, ext string) (domain.WorkProduct, bool, error) {
	for _, w := range m.all {
		if w.IssueID == issueID && w.Type == typ && w.ExternalID == ext {
			return w, true, nil
		}
	}
	return domain.WorkProduct{}, false, nil
}

func (m *memWorkProducts) SaveWorkProduct(_ context.Context, w domain.WorkProduct) (domain.WorkProduct, error) {
	if w.ID == 0 {
		w.ID = uint64(len(m.all) + 1)
		m.all = append(m.all, w)
		return w, nil
	}
	m.all[w.ID-1] = w
	return w, nil
}

func TestOpenPullRequest(t *testing.T) {
	ctx := context.Background()
	act := &memActivity{}
	s := NewService(nil, &memIssues{byID: map[uint64]domain.Issue{}}, nil, nil, memberGuild{}, memProjects{}, act, nil, nil)
	s.Logf = t.Logf
	s.Agents = func(_ context.Context, _ uint64, ids []uint64) (map[uint64]AssigneeAgent, error) {
		return map[uint64]AssigneeAgent{3: {Name: "Ada"}}, nil
	}
	s.RunsLive = func(_ context.Context, ids []uint64) (map[uint64]bool, error) {
		return map[uint64]bool{30: true, 31: true}, nil
	}
	products := &memWorkProducts{}
	s.WorkProducts = products
	var asked []string
	refuse := error(nil)
	s.PullRequests = func(_ context.Context, applicationID uint64, head, title, body string) (OpenedPullRequest, error) {
		asked = append(asked, head+"|"+title+"|"+body)
		if refuse != nil {
			return OpenedPullRequest{}, refuse
		}
		return OpenedPullRequest{Provider: "forgejo", Number: 4, URL: "http://git/pr/4", Title: title}, nil
	}
	all := func(ids []uint64) ([]uint64, error) { return ids, nil }
	issueURL := func(id string) string { return "http://bakery/#/issues/" + id }
	open := func(by domain.Actor, in PullRequestInput) (domain.WorkProduct, bool, error) {
		return s.OpenPullRequest(ctx, 1, by, "1", in, issueURL, all)
	}

	if _, err := s.CreateIssue(ctx, 1, domain.ByMember(7), IssueInput{Title: "Fix it", Status: "todo", AssigneeAgentID: 3, ProjectID: 1}, all); err != nil {
		t.Fatal(err)
	}
	var refused *PullRequestRefusedError
	if _, _, err := open(domain.ByMember(7), PullRequestInput{}); !errors.As(err, &refused) {
		t.Fatalf("no application: %v", err)
	}
	if _, err := s.ChangeIssue(ctx, 1, domain.ByMember(7), "1", IssuePatch{ApplicationID: ptr(uint64(10))}, all); err != nil {
		t.Fatal(err)
	}
	runA, runB := domain.Actor{AgentID: 3, RunID: 30}, domain.Actor{AgentID: 3, RunID: 31}
	if _, _, err := open(runA, PullRequestInput{}); !errors.Is(err, domain.ErrNotHolder) {
		t.Fatalf("a run without the checkout: %v", err)
	}
	if _, err := s.CheckoutIssue(ctx, 1, runA, "1", nil, all); err != nil {
		t.Fatal(err)
	}
	var held *domain.HeldError
	if _, _, err := open(runB, PullRequestInput{}); !errors.As(err, &held) {
		t.Fatalf("another run: %v", err)
	}
	for _, c := range []struct {
		err  error
		want string
	}{
		{ErrPullRequestNoToken, "The application's webhook has no git host token"},
		{ErrPullRequestNotPushed, "Push the branch bakery/bak-1 first"},
	} {
		refuse = c.err
		if _, _, err := open(runA, PullRequestInput{}); !errors.As(err, &refused) || refused.Message != c.want {
			t.Errorf("%v: %v", c.err, err)
		}
	}
	refuse = nil

	w, created, err := open(runA, PullRequestInput{})
	if err != nil || !created || w.ExternalID != "4" || w.Status != domain.PullRequestOpen || w.CreatedBy != runA || w.ApplicationID != 10 {
		t.Fatalf("open: %+v %v %v", w, created, err)
	}
	if got := asked[len(asked)-1]; got != "bakery/bak-1|BAK-1 Fix it|Opened by The Bakery for [BAK-1](http://bakery/#/issues/BAK-1)." {
		t.Errorf("asked %q", got)
	}
	if !slices.Contains(act.actions, domain.PullRequestOpenedAction) {
		t.Errorf("activity %v", act.actions)
	}
	n := len(act.actions)
	// A second call answers the same one; a person may open it too.
	w2, created, err := open(domain.ByMember(7), PullRequestInput{Title: "Mine", Body: "Body"})
	if err != nil || created || w2.ID != w.ID || len(act.actions) != n {
		t.Fatalf("again: %+v %v %v", w2, created, err)
	}
	if got := asked[len(asked)-1]; got != "bakery/bak-1|Mine|Body" {
		t.Errorf("asked %q", got)
	}
	// One recorded closed that the git host answers open is open again.
	products.all[0].Status = domain.PullRequestClosed
	if w, _, err = open(runA, PullRequestInput{}); err != nil || w.Status != domain.PullRequestOpen {
		t.Fatalf("reopened: %+v %v", w, err)
	}
	if ws, _ := s.IssueWorkProducts(ctx, w.IssueID); len(ws) != 1 {
		t.Errorf("work products %+v", ws)
	}
}
