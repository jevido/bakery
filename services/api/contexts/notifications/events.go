package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/databases"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// subscribe registers the translations of what other contexts announce
// into Notifications.
func subscribe() {
	deployments.OnDeploymentFinished(func(ctx context.Context, e deployments.DeploymentFinished) {
		svc().Notify(ctx, deploymentNotification(e, DashboardURL()))
	})
	databases.OnBackupFinished(func(ctx context.Context, e databases.BackupFinished) {
		svc().Notify(ctx, backupNotification(e, DashboardURL()))
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

// size is n bytes for people: 1.4 MB.
func size(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

func backupNotification(e databases.BackupFinished, dashboard string) domain.Notification {
	n := domain.Notification{
		Kind:  domain.BackupSucceeded,
		Title: "Backup of " + e.DatabaseName + " succeeded",
		Link:  fmt.Sprintf("%s/#/databases/%d", dashboard, e.DatabaseID),
		At:    e.FinishedAt,
	}
	trigger := "A scheduled backup"
	if e.Trigger == "manual" {
		trigger = "A backup started by hand"
	}
	where := "kept on the server's disk"
	if e.OffSite {
		where = "kept on the server's disk and uploaded to S3"
	}
	if e.Succeeded {
		n.Body = fmt.Sprintf("%s of %s: %s, %s.", trigger, e.Engine, size(e.SizeBytes), where)
		return n
	}
	n.Kind = domain.BackupFailed
	n.Title = "Backup of " + e.DatabaseName + " failed"
	n.Body = e.Reason + "\n" + trigger + " of " + e.Engine + "."
	return n
}
