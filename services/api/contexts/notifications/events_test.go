package notifications

import (
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
	"github.com/jevido/bakery/services/api/contexts/servers"
)

func TestDeploymentNotification(t *testing.T) {
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	n := deploymentNotification(deployments.DeploymentFinished{
		ApplicationID: 3, ApplicationSlug: "shop", Reason: "build failed: exit status 1", Branch: "main",
		CommitSHA: "0123456789abcdef", CommitMessage: "Fix the login", Trigger: "webhook", FinishedAt: at,
	}, "https://bakery.example.com")
	want := domain.Notification{
		Kind: domain.DeploymentFailure, Title: "Deployment of shop failed",
		Body: "build failed: exit status 1\nBranch main, commit 0123456789ab: Fix the login\nStarted by a push.",
		Link: "https://bakery.example.com/#/applications/3", At: at,
	}
	if n != want {
		t.Fatalf("got %+v\nwant %+v", n, want)
	}
	n = deploymentNotification(deployments.DeploymentFinished{ApplicationID: 3, ApplicationSlug: "shop", Succeeded: true, Rollback: true}, "x")
	if n.Kind != domain.DeploymentSuccess || n.Title != "Deployment of shop succeeded" || n.Body != "A rollback to an earlier deployment." {
		t.Fatalf("got %+v", n)
	}
	n = deploymentNotification(deployments.DeploymentFinished{ApplicationID: 3, ApplicationSlug: "shop", Succeeded: true, Preview: 7, Branch: "feature", CommitSHA: "abc", Trigger: "webhook"}, "x")
	if n.Title != "Deployment of shop (preview of pull request #7) succeeded" || n.Body != "Branch feature, commit abc\nStarted by the pull request." {
		t.Fatalf("preview: %+v", n)
	}
}

func TestBackupNotification(t *testing.T) {
	n := backupNotification(databases.BackupExecutionFinished{
		DatabaseID: 4, DatabaseName: "main", Type: "postgresql", Reason: "the dump exited with code 1: refused", Trigger: "scheduled",
	}, "http://localhost:4930")
	if n.Kind != domain.BackupFailure || n.Title != "Backup of main failed" || n.Link != "http://localhost:4930/#/databases/4" ||
		n.Body != "the dump exited with code 1: refused\nA scheduled backup of postgresql." {
		t.Fatalf("got %+v", n)
	}
	n = backupNotification(databases.BackupExecutionFinished{DatabaseName: "main", Type: "mysql", Succeeded: true, Trigger: "manual", SizeBytes: 1_400_000, OffSite: true}, "x")
	if n.Kind != domain.BackupSuccess || n.Body != "A backup started by hand of mysql: 1.4 MB, kept on the server's disk and uploaded to S3." {
		t.Fatalf("got %+v", n)
	}
}

func TestServerNotification(t *testing.T) {
	now := time.Now()
	n, ok := serverNotification(servers.ServerHealthChanged{ServerID: 2, ServerName: "edge-1", Change: "unreachable", Reason: "dial tcp: i/o timeout"}, "x", now)
	if !ok || n.Kind != domain.ServerUnreachable || n.Title != "Server edge-1 is unreachable" || n.Link != "x/#/servers/2" {
		t.Fatalf("got %+v", n)
	}
	n, _ = serverNotification(servers.ServerHealthChanged{ServerName: "edge-1", Change: "server_disk_usage", DiskUsed: 93_000_000_000, DiskTotal: 100_000_000_000}, "x", now)
	if n.Kind != domain.ServerDiskUsage || n.Body != "93.0 GB of 100.0 GB used (93 %). Clean up on its Server page removes images nothing needs any more." {
		t.Fatalf("got %+v", n)
	}
	if _, ok := serverNotification(servers.ServerHealthChanged{Change: "rebooted"}, "x", now); ok {
		t.Fatal("unknown change notified")
	}
}
