package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

// PullRequestEvent is a Pull request of an Application's repository
// opened (or reopened), pushed to or closed, as deployments tells it.
type PullRequestEvent struct {
	GuildID       uint64
	ApplicationID uint64
	Provider      string
	Number        int
	// Action is "opened", "pushed" or "closed".
	Action string
	Merged bool
	// Branch is the head branch.
	Branch string
	URL    string
	Title  string
}

// FollowPullRequest moves the pull_request Work products of the
// Application and number as the event says. An opened one whose head is
// the Agent branch of an Issue of the Guild with that Application, which
// has none yet, is recorded: someone opened it by hand. The Bakery itself
// is the Actor of what it records.
func (s *Service) FollowPullRequest(ctx context.Context, e PullRequestEvent) error {
	if s.WorkProducts == nil {
		return nil
	}
	ext := fmt.Sprint(e.Number)
	products, err := s.WorkProducts.ApplicationWorkProducts(ctx, e.ApplicationID, domain.PullRequestProduct, ext)
	if err != nil {
		return err
	}
	var followed bool
	for _, w := range products {
		if w.GuildID != e.GuildID {
			continue
		}
		followed = true
		if err := s.followPullRequest(ctx, w, e); err != nil {
			return err
		}
	}
	if followed || e.Action != "opened" {
		return nil
	}
	i, found, err := s.issueOfAgentBranch(ctx, e.GuildID, e.Branch)
	if err != nil || !found || i.ApplicationID != e.ApplicationID {
		return err
	}
	if _, found, err := s.WorkProducts.WorkProduct(ctx, i.ID, domain.PullRequestProduct, ext); err != nil || found {
		return err
	}
	w, err := s.WorkProducts.SaveWorkProduct(ctx, domain.NewPullRequest(i, e.ApplicationID, e.Provider, e.Number, e.Title, e.URL, domain.Actor{}))
	if err != nil {
		return err
	}
	s.publish(ctx, domain.PullRequestOpened{Happened: s.happened(domain.Actor{}), Issue: i, WorkProduct: w})
	return nil
}

func (s *Service) followPullRequest(ctx context.Context, w domain.WorkProduct, e PullRequestEvent) error {
	to := w.Status
	switch {
	case e.Action == "closed" && e.Merged:
		to = domain.PullRequestMerged
	case e.Action == "closed":
		to = domain.PullRequestClosed
	case e.Action == "opened":
		to = domain.PullRequestOpen
	}
	changed, err := w.Move(to)
	if err != nil {
		// A merged one never changes again; a late event cannot undo it.
		return nil
	}
	if e.Title != "" {
		w.Title = e.Title
	}
	if e.URL != "" {
		w.URL = e.URL
	}
	// Saved on every event, a push included, so updated_at tells when the
	// Pull request last moved.
	if w, err = s.WorkProducts.SaveWorkProduct(ctx, w); err != nil || !changed {
		return err
	}
	i, found, err := s.issues.Issue(ctx, w.IssueID)
	if err != nil || !found {
		return err
	}
	if e := (domain.WorkProductMoved{Happened: s.happened(domain.Actor{}), Issue: i, WorkProduct: w}); e.Recorded() {
		s.publish(ctx, e)
	}
	return nil
}

// issueOfAgentBranch is the Guild's Issue whose Agent branch is branch.
func (s *Service) issueOfAgentBranch(ctx context.Context, guildID uint64, branch string) (domain.Issue, bool, error) {
	identifier, ok := strings.CutPrefix(branch, domain.AgentBranch(""))
	if !ok {
		return domain.Issue{}, false, nil
	}
	prefix, number, ok := domain.ParseIdentifier(identifier)
	if !ok {
		return domain.Issue{}, false, nil
	}
	own, err := s.guilds.IssuePrefix(ctx, guildID)
	if err != nil || !strings.EqualFold(prefix, own) {
		return domain.Issue{}, false, err
	}
	i, found, err := s.issues.IssueByNumber(ctx, guildID, number)
	if err != nil || !found || i.GuildID != guildID || domain.AgentBranch(domain.Identifier(own, i.Number)) != branch {
		return domain.Issue{}, false, err
	}
	return i, true, nil
}

// PreviewEvent is the Preview of an Application's Pull request number
// deploying, ready at URL, failed or removed, as deployments tells it.
// GuildID is 0 when deployments does not tell it.
type PreviewEvent struct {
	GuildID       uint64
	ApplicationID uint64
	Number        int
	Status        domain.WorkProductStatus
	URL           string
}

// FollowPreview keeps the preview_url Work product beside every
// pull_request one of the Application and number up to date, making it
// the first time the Preview deploys. The Bakery itself is the Actor.
func (s *Service) FollowPreview(ctx context.Context, e PreviewEvent) error {
	if s.WorkProducts == nil {
		return nil
	}
	ext := fmt.Sprint(e.Number)
	prs, err := s.WorkProducts.ApplicationWorkProducts(ctx, e.ApplicationID, domain.PullRequestProduct, ext)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		if e.GuildID != 0 && pr.GuildID != e.GuildID {
			continue
		}
		w, found, err := s.WorkProducts.WorkProduct(ctx, pr.IssueID, domain.PreviewURLProduct, ext)
		if err != nil {
			return err
		}
		changed := true
		switch {
		case !found && e.Status == domain.PreviewRemoved:
			// Nothing to show for a Preview the Issue never saw.
			continue
		case !found:
			w = domain.NewPreviewURL(pr, e.URL, e.Status)
		default:
			if changed, err = w.Move(e.Status); err != nil {
				continue
			}
			if e.URL != "" && e.URL != w.URL {
				w.URL, changed = e.URL, true
			}
		}
		if !changed {
			continue
		}
		if w, err = s.WorkProducts.SaveWorkProduct(ctx, w); err != nil {
			return err
		}
		moved := domain.WorkProductMoved{Happened: s.happened(domain.Actor{}), WorkProduct: w}
		if !moved.Recorded() {
			continue
		}
		i, found, err := s.issues.Issue(ctx, w.IssueID)
		if err != nil {
			return err
		}
		if found {
			moved.Issue = i
			s.publish(ctx, moved)
		}
	}
	return nil
}
