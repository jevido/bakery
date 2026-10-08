package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type fakeHosts struct {
	open    map[string]OpenedPullRequest
	pushed  map[string]bool
	repo    domain.Repository
	created int
}

func (f *fakeHosts) FindPullRequest(_ context.Context, r domain.Repository, _, head, base string) (OpenedPullRequest, bool, error) {
	f.repo = r
	pr, ok := f.open[head+">"+base]
	return pr, ok, nil
}

func (f *fakeHosts) BranchExists(_ context.Context, _ domain.Repository, _, branch string) (bool, error) {
	return f.pushed[branch], nil
}

func (f *fakeHosts) CreatePullRequest(_ context.Context, r domain.Repository, token, head, base, title, _ string) (OpenedPullRequest, error) {
	if token != "tok" {
		return OpenedPullRequest{}, errors.New("wrong token")
	}
	if !f.pushed[head] {
		return OpenedPullRequest{}, errors.New("forgejo answered 404")
	}
	f.created++
	pr := OpenedPullRequest{Provider: r.Provider, Number: 4, URL: "https://git/pr/4", Title: title}
	f.open[head+">"+base] = pr
	return pr, nil
}

func TestOpenPullRequest(t *testing.T) {
	ctx := context.Background()
	s := newSetup(t, fakeCloner{})
	// Cloned over SSH: the REST base comes from the Webhook's last call.
	s.app = func(a *Application) { a.GitURL = "ssh://git@git.example.com:2222/u/r.git" }
	store := memWebhooks{}
	hooks := NewWebhooks(s.service, store)
	hosts := &fakeHosts{open: map[string]OpenedPullRequest{}, pushed: map[string]bool{"bakery/def-12": true}}
	hooks.Hosts = hosts

	if _, err := hooks.OpenPullRequest(ctx, 1, "bakery/def-12", "DEF-12 Fix", ""); !errors.Is(err, ErrNoGitHostToken) {
		t.Fatalf("no webhook: %v", err)
	}
	hook, _ := hooks.Webhook(ctx, 1)
	if _, err := hooks.OpenPullRequest(ctx, 1, "bakery/def-12", "DEF-12 Fix", ""); !errors.Is(err, ErrNoGitHostToken) {
		t.Fatalf("no token: %v", err)
	}
	on, token := true, "tok"
	hooks.SetPreviews(ctx, 1, &on, &token)
	if _, err := hooks.OpenPullRequest(ctx, 1, "bakery/def-12", "DEF-12 Fix", ""); !errors.Is(err, domain.ErrUnknownGitHost) {
		t.Fatalf("before a call: %v", err)
	}

	// A verified call remembers its Provider and the repository's REST
	// base; Save keeps both.
	h, body := forgejoPullRequest(hook.Secret, "edited", 1, "feature", "main", 5)
	if _, err := hooks.ReceivePush(ctx, 1, h, body); err != nil {
		t.Fatal(err)
	}
	hooks.SetAutoDeploy(ctx, 1, false)
	if got, _ := hooks.Webhook(ctx, 1); got.Provider != domain.Forgejo || got.RepositoryAPI != "http://git/api/v1/repos/u/r" {
		t.Fatalf("provider: %+v", got)
	}

	if _, err := hooks.OpenPullRequest(ctx, 1, "bakery/def-13", "DEF-13", ""); !errors.Is(err, ErrBranchNotPushed) {
		t.Fatalf("unpushed: %v", err)
	}
	pr, err := hooks.OpenPullRequest(ctx, 1, "bakery/def-12", "DEF-12 Fix", "")
	if err != nil || !pr.Created || pr.Number != 4 || hosts.repo.API != "http://git/api/v1/repos/u/r" || hosts.repo.Owner != "u" {
		t.Fatalf("open: %+v %v %+v", pr, err, hosts.repo)
	}
	pr, err = hooks.OpenPullRequest(ctx, 1, "bakery/def-12", "DEF-12 Fix", "")
	if err != nil || pr.Created || pr.Number != 4 || hosts.created != 1 {
		t.Fatalf("again: %+v %v", pr, err)
	}

	s.app = func(a *Application) { a.BuildPack, a.GitURL = BuildPackDockerImage, "" }
	if _, err := hooks.OpenPullRequest(ctx, 1, "bakery/def-12", "x", ""); !errors.Is(err, ErrNoRepository) {
		t.Fatalf("image: %v", err)
	}
}
