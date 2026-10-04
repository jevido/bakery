package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/identity"
	"github.com/jevido/bakery/services/api/contexts/notifications/app"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
	"github.com/jevido/bakery/services/api/contexts/servers"
)

// subscribe registers the translations of what other contexts announce
// into Notifications.
func subscribe() {
	deployments.OnDeploymentFinished(func(ctx context.Context, e deployments.DeploymentFinished) {
		svc().Notify(ctx, deploymentNotification(e, DashboardURL()))
	})
	databases.OnBackupExecutionFinished(func(ctx context.Context, e databases.BackupExecutionFinished) {
		svc().Notify(ctx, backupNotification(e, DashboardURL()))
	})
	identity.OnInvitationCreated(func(ctx context.Context, e identity.InvitationCreated) (bool, error) {
		return svc().SendInvitation(ctx, app.Invitation(e))
	})
	servers.OnServerHealthChanged(func(ctx context.Context, e servers.ServerHealthChanged) {
		if n, ok := serverNotification(e, DashboardURL(), time.Now()); ok {
			svc().Notify(ctx, n)
		}
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
	if e.Preview != 0 {
		name += fmt.Sprintf(" (preview of pull request #%d)", e.Preview)
	}
	n := domain.Notification{
		Kind:  domain.DeploymentSuccess,
		Title: "Deployment of " + name + " succeeded",
		Link:  fmt.Sprintf("%s/#/applications/%d", dashboard, e.ApplicationID),
		At:    e.FinishedAt,
	}
	var body []string
	if !e.Succeeded {
		n.Kind = domain.DeploymentFailure
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
	switch {
	case e.Trigger == "webhook" && e.Preview != 0:
		body = append(body, "Started by the pull request.")
	case e.Trigger == "webhook":
		body = append(body, "Started by a push.")
	case e.Trigger == "restart":
		body = append(body, "A restart without rebuilding.")
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

func backupNotification(e databases.BackupExecutionFinished, dashboard string) domain.Notification {
	n := domain.Notification{
		Kind:  domain.BackupSuccess,
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
		n.Body = fmt.Sprintf("%s of %s: %s, %s.", trigger, e.Type, size(e.SizeBytes), where)
		return n
	}
	n.Kind = domain.BackupFailure
	n.Title = "Backup of " + e.DatabaseName + " failed"
	n.Body = e.Reason + "\n" + trigger + " of " + e.Type + "."
	return n
}

func serverNotification(e servers.ServerHealthChanged, dashboard string, now time.Time) (domain.Notification, bool) {
	n := domain.Notification{Link: fmt.Sprintf("%s/#/servers/%d", dashboard, e.ServerID), At: now}
	switch e.Change {
	case "unreachable":
		n.Kind = domain.ServerUnreachable
		n.Title = "Server " + e.ServerName + " is unreachable"
		n.Body = "The Bakery could not reach it twice in a row: " + e.Reason + "\nApplications on it may be down, and deployments to it fail until it is back."
	case "reachable":
		n.Kind = domain.ServerReachable
		n.Title = "Server " + e.ServerName + " is reachable again"
		n.Body = "The Bakery reaches it again."
	case "server_disk_usage":
		n.Kind = domain.ServerDiskUsage
		n.Title = "High disk usage on " + e.ServerName
		percent := int64(0)
		if e.DiskTotal > 0 {
			percent = e.DiskUsed * 100 / e.DiskTotal
		}
		n.Body = fmt.Sprintf("%s of %s used (%d %%). Clean up on its Server page removes images nothing needs any more.", size(e.DiskUsed), size(e.DiskTotal), percent)
	default:
		return n, false
	}
	return n, true
}
