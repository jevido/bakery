package domain

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// PullRequestAction is what happened to a Pull request, in Bakery's words.
type PullRequestAction string

const (
	// PullRequestOpened is opened or reopened.
	PullRequestOpened PullRequestAction = "opened"
	// PullRequestPushed is new commits on its head branch.
	PullRequestPushed PullRequestAction = "pushed"
	// PullRequestClosed is closed or merged.
	PullRequestClosed PullRequestAction = "closed"
)

// PullRequest is a Pull request event, translated from its Provider's
// payload. Action is empty for events Previews ignore (edited, labelled,
// review requested, ...).
type PullRequest struct {
	Number int
	Action PullRequestAction
	// Branch is the head branch, Base the branch it merges into.
	Branch string
	Base   string
	Title  string
	URL    string
	// Merged is true for a Pull request closed by merging it.
	Merged bool
	// SameRepo is false for a Pull request from a fork.
	SameRepo bool
	// API is the Provider's REST base for the repository, where the
	// Preview comment goes.
	API string
}

var ErrNotAPullRequest = errors.New("not a pull request event")

// IsPullRequest reports whether event is the Provider's Pull request event.
func IsPullRequest(p Provider, event string) bool {
	if p == GitLab {
		return event == "Merge Request Hook"
	}
	return event == "pull_request"
}

// ParsePullRequest reads a Pull request event of the Provider.
func ParsePullRequest(p Provider, body []byte) (PullRequest, error) {
	switch p {
	case GitHub:
		return parseGitHub(body)
	case Gitea, Forgejo:
		return parseGitea(body)
	case GitLab:
		return parseGitLab(body)
	}
	return PullRequest{}, ErrNotAPullRequest
}

type branchRef struct {
	Ref    string `json:"ref"`
	RepoID int64  `json:"repo_id"`
	Repo   *struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	} `json:"repo"`
}

func (b branchRef) repoName() string {
	if b.Repo == nil {
		return ""
	}
	return b.Repo.FullName
}

func parseGitHub(body []byte) (PullRequest, error) {
	var e struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest *struct {
			HTMLURL string    `json:"html_url"`
			Title   string    `json:"title"`
			Merged  bool      `json:"merged"`
			Head    branchRef `json:"head"`
			Base    branchRef `json:"base"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
			URL      string `json:"url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return PullRequest{}, err
	}
	if e.PullRequest == nil || e.Number == 0 {
		return PullRequest{}, ErrNotAPullRequest
	}
	pr := e.PullRequest
	return PullRequest{
		Number: e.Number, Action: map[string]PullRequestAction{"opened": PullRequestOpened, "reopened": PullRequestOpened, "synchronize": PullRequestPushed, "closed": PullRequestClosed}[e.Action],
		Branch: pr.Head.Ref, Base: pr.Base.Ref, Title: pr.Title, URL: pr.HTMLURL, Merged: pr.Merged,
		SameRepo: pr.Head.repoName() != "" && pr.Head.repoName() == e.Repository.FullName,
		API:      strings.TrimSuffix(e.Repository.URL, "/"),
	}, nil
}

func parseGitea(body []byte) (PullRequest, error) {
	var e struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest *struct {
			HTMLURL string    `json:"html_url"`
			Title   string    `json:"title"`
			Merged  bool      `json:"merged"`
			Head    branchRef `json:"head"`
			Base    branchRef `json:"base"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return PullRequest{}, err
	}
	if e.PullRequest == nil || e.Number == 0 {
		return PullRequest{}, ErrNotAPullRequest
	}
	pr := e.PullRequest
	headRepo, baseRepo := pr.Head.RepoID, pr.Base.RepoID
	if headRepo == 0 && pr.Head.Repo != nil {
		headRepo = pr.Head.Repo.ID
	}
	if baseRepo == 0 && pr.Base.Repo != nil {
		baseRepo = pr.Base.Repo.ID
	}
	api := ""
	if u, err := url.Parse(e.Repository.HTMLURL); err == nil && u.Host != "" {
		// html_url is <root>/<owner>/<repo>; the root may have a path.
		root := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/"+e.Repository.FullName)
		api = u.Scheme + "://" + u.Host + root + "/api/v1/repos/" + e.Repository.FullName
	}
	return PullRequest{
		Number: e.Number, Action: map[string]PullRequestAction{"opened": PullRequestOpened, "reopened": PullRequestOpened, "synchronized": PullRequestPushed, "closed": PullRequestClosed}[e.Action],
		Branch: pr.Head.Ref, Base: pr.Base.Ref, Title: pr.Title, URL: pr.HTMLURL, Merged: pr.Merged,
		SameRepo: headRepo != 0 && headRepo == baseRepo, API: api,
	}, nil
}

func parseGitLab(body []byte) (PullRequest, error) {
	var e struct {
		ObjectKind       string `json:"object_kind"`
		ObjectAttributes *struct {
			IID             int    `json:"iid"`
			Action          string `json:"action"`
			SourceBranch    string `json:"source_branch"`
			TargetBranch    string `json:"target_branch"`
			SourceProjectID int64  `json:"source_project_id"`
			TargetProjectID int64  `json:"target_project_id"`
			URL             string `json:"url"`
			Title           string `json:"title"`
			OldRev          string `json:"oldrev"`
		} `json:"object_attributes"`
		Project struct {
			ID     int64  `json:"id"`
			WebURL string `json:"web_url"`
		} `json:"project"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return PullRequest{}, err
	}
	a := e.ObjectAttributes
	if e.ObjectKind != "merge_request" || a == nil || a.IID == 0 {
		return PullRequest{}, ErrNotAPullRequest
	}
	var action PullRequestAction
	switch a.Action {
	case "open", "reopen":
		action = PullRequestOpened
	case "update":
		// An update without oldrev changed the title or labels, not the code.
		if a.OldRev != "" {
			action = PullRequestPushed
		}
	case "close", "merge":
		action = PullRequestClosed
	}
	api := ""
	if u, err := url.Parse(e.Project.WebURL); err == nil && u.Host != "" {
		api = u.Scheme + "://" + u.Host + "/api/v4/projects/" + strconv.FormatInt(e.Project.ID, 10)
	}
	return PullRequest{
		Number: a.IID, Action: action, Merged: a.Action == "merge", Branch: a.SourceBranch, Base: a.TargetBranch, Title: a.Title, URL: a.URL,
		SameRepo: a.SourceProjectID != 0 && a.SourceProjectID == a.TargetProjectID, API: api,
	}, nil
}
