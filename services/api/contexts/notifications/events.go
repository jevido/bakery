package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// subscribe registers the translations of what other contexts announce
// into Notifications.
func subscribe() {
	deployments.OnDeploymentFinished(func(ctx context.Context, e deployments.DeploymentFinished) {
		svc().Notify(ctx, deploymentNotification(e, DashboardURL()))
	})
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func deploymentNotification(e deployments.DeploymentFinished, dashboard string) domain.Notification {
	name := e.ApplicationSlug
	if name == "" {
		name = fmt.Sprintf("application %d", e.ApplicationID)
	}
	n := domain.Notification{
		Kind:  domain.DeploymentSucceeded,
		Title: "Deployment of " + name + " succeeded",
		Link:  fmt.Sprintf("%s/#/applications/%d", dashboard, e.ApplicationID),
		At:    e.FinishedAt,
	}
	var body []string
	if !e.Succeeded {
		n.Kind = domain.DeploymentFailed
		n.Title = "Deployment of " + name + " failed"
		body = append(body, e.Reason)
	}
	switch {
	case e.Rollback:
		body = append(body, "A rollback to an earlier deployment.")
	case e.CommitSHA != "":
		line := "Branch " + e.Branch + ", commit " + shortSHA(e.CommitSHA)
		if e.CommitMessage != "" {
			line += ": " + e.CommitMessage
		}
		body = append(body, line)
	}
	if e.Trigger == "webhook" {
		body = append(body, "Started by a push.")
	}
	n.Body = strings.Join(body, "\n")
	return n
}
