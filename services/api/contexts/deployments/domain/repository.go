package domain

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// ErrUnknownGitHost is a repository whose Provider The Bakery cannot tell:
// its Webhook has not had a verified call yet and the host is neither
// github.com nor a GitLab host.
var ErrUnknownGitHost = errors.New("the git host of this repository is not known yet")

// Repository is an Application's repository as its git host's REST API
// sees it.
type Repository struct {
	Provider Provider
	// API is the repository's REST base, e.g.
	// https://api.github.com/repos/acme/shop.
	API string
	// Owner is the account or group the repository is in; GitHub names a
	// head branch "<owner>:<branch>".
	Owner string
}

// RepositoryOf works out the Repository behind the Application's git URL.
// remembered is the Provider its Webhook was last called by, empty when it
// has not been; then github.com is GitHub and a host with "gitlab" in its
// name GitLab. HTTP(S) URLs keep their scheme, host and port; SSH URLs
// (git@host:owner/repo.git, ssh://git@host:2222/owner/repo.git) map to
// https://host, since the REST API is not on the SSH port.
func RepositoryOf(remembered Provider, gitURL string) (Repository, error) {
	base, path, err := splitGitURL(gitURL)
	if err != nil {
		return Repository{}, err
	}
	host := strings.ToLower(hostOf(base))
	p := remembered
	if p == "" {
		switch {
		case host == "github.com" || host == "www.github.com":
			p = GitHub
		case strings.Contains(host, "gitlab"):
			p = GitLab
		default:
			return Repository{}, ErrUnknownGitHost
		}
	}
	segments := strings.Split(path, "/")
	switch p {
	case GitLab:
		return Repository{Provider: p, API: base + "/api/v4/projects/" + url.PathEscape(path), Owner: strings.Join(segments[:len(segments)-1], "/")}, nil
	case GitHub, Gitea, Forgejo:
		// The last two segments are owner and repository; anything before
		// them is the path the git host is served under.
		owner, repo := segments[len(segments)-2], segments[len(segments)-1]
		prefix := strings.Join(segments[:len(segments)-2], "/")
		if prefix != "" {
			prefix = "/" + prefix
		}
		if p != GitHub {
			return Repository{Provider: p, API: base + prefix + "/api/v1/repos/" + owner + "/" + repo, Owner: owner}, nil
		}
		if host == "github.com" || host == "www.github.com" {
			return Repository{Provider: p, API: "https://api.github.com/repos/" + owner + "/" + repo, Owner: owner}, nil
		}
		// GitHub Enterprise Server.
		return Repository{Provider: p, API: base + prefix + "/api/v3/repos/" + owner + "/" + repo, Owner: owner}, nil
	}
	return Repository{}, ErrUnknownGitHost
}

// splitGitURL splits a git URL into the web base of its host
// (scheme://host[:port]) and the repository path without ".git".
func splitGitURL(gitURL string) (base, path string, err error) {
	bad := errors.New("not a git repository URL: " + gitURL)
	s := strings.TrimSpace(gitURL)
	var host, rest string
	if !strings.Contains(s, "://") {
		// scp-like: [user@]host:owner/repo.git
		at := strings.LastIndex(s, "@")
		colon := strings.Index(s[at+1:], ":")
		if colon < 0 {
			return "", "", bad
		}
		host, rest = s[at+1:at+1+colon], s[at+1+colon+1:]
		base = "https://" + host
	} else {
		u, perr := url.Parse(s)
		if perr != nil || u.Host == "" {
			return "", "", bad
		}
		switch u.Scheme {
		case "http", "https":
			base = u.Scheme + "://" + u.Host
		case "ssh", "git+ssh", "git":
			base = "https://" + u.Hostname()
		default:
			return "", "", bad
		}
		host, rest = u.Hostname(), u.Path
	}
	path = strings.TrimSuffix(strings.Trim(rest, "/"), ".git")
	path = strings.Trim(path, "/")
	if host == "" || strings.Count(path, "/") < 1 || strings.Contains(path, "//") {
		return "", "", bad
	}
	return base, path, nil
}

func hostOf(base string) string {
	h := base[strings.Index(base, "://")+3:]
	if i := strings.LastIndex(h, ":"); i >= 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	return h
}

// RepositoryAPI reads the repository's REST base from any verified call of
// the Provider (push, Pull request, ping), or "" when the body names none.
// Unlike the git URL it is where the git host says its web UI is, so it
// holds for repositories cloned over SSH from a host whose web UI is not
// on 443.
func RepositoryAPI(p Provider, body []byte) string {
	var e struct {
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
			URL      string `json:"url"`
			PullsURL string `json:"pulls_url"`
		} `json:"repository"`
		Project struct {
			ID     int64  `json:"id"`
			WebURL string `json:"web_url"`
		} `json:"project"`
	}
	if json.Unmarshal(body, &e) != nil {
		return ""
	}
	switch p {
	case GitHub:
		// pulls_url is in push and Pull request events alike;
		// repository.url is the web URL in push events and the API in
		// Pull request events.
		if i := strings.Index(e.Repository.PullsURL, "/pulls{"); i > 0 {
			return e.Repository.PullsURL[:i]
		}
		return ""
	case Gitea, Forgejo:
		return giteaAPI(e.Repository.HTMLURL, e.Repository.FullName)
	case GitLab:
		return gitlabAPI(e.Project.WebURL, e.Project.ID)
	}
	return ""
}

// giteaAPI is the REST base of a Gitea or Forgejo repository from its
// html_url, <root>/<owner>/<repo>, where the root may have a path.
func giteaAPI(htmlURL, fullName string) string {
	u, err := url.Parse(htmlURL)
	if err != nil || u.Host == "" || fullName == "" {
		return ""
	}
	root := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/"+fullName)
	return u.Scheme + "://" + u.Host + root + "/api/v1/repos/" + fullName
}

// gitlabAPI is the REST base of a GitLab project from its web_url and id.
func gitlabAPI(webURL string, id int64) string {
	u, err := url.Parse(webURL)
	if err != nil || u.Host == "" || id == 0 {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/api/v4/projects/" + strconv.FormatInt(id, 10)
}
