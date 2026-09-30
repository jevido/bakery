package notifications

import (
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases"
	"github.com/jevido/bakery/services/api/contexts/deployments"
	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

func TestDeploymentNotification(t *testing.T) {
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	n := deploymentNotification(deployments.DeploymentFinished{
		ApplicationID: 3, ApplicationSlug: "shop", Reason: "build failed: exit status 1", Branch: "main",
		CommitSHA: "0123456789abcdef", CommitMessage: "Fix the login", Trigger: "webhook", FinishedAt: at,
	}, "https://bakery.example.com")
	want := domain.Notification{
		Kind: domain.DeploymentFailed, Title: "Deployment of shop failed",
		Body: "build failed: exit status 1\nBranch main, commit 0123456789ab: Fix the login\nStarted by a push.",
		Link: "https://bakery.example.com/#/applications/3", At: at,
	}
	if n != want {
		t.Fatalf("got %+v\nwant %+v", n, want)
	}
	n = deploymentNotification(deployments.DeploymentFinished{ApplicationID: 3, ApplicationSlug: "shop", Succeeded: true, Rollback: true}, "x")
	if n.Kind != domain.DeploymentSucceeded || n.Title != "Deployment of shop succeeded" || n.Body != "A rollback to an earlier deployment." {
		t.Fatalf("got %+v", n)
	}
}

func TestBackupNotification(t *testing.T) {
	n := backupNotification(databases.BackupFinished{
		DatabaseID: 4, DatabaseName: "main", Engine: "postgresql", Reason: "the dump exited with code 1: refused", Trigger: "scheduled",
	}, "http://localhost:4930")
	if n.Kind != domain.BackupFailed || n.Title != "Backup of main failed" || n.Link != "http://localhost:4930/#/databases/4" ||
		n.Body != "the dump exited with code 1: refused\nA scheduled backup of postgresql." {
		t.Fatalf("got %+v", n)
	}
	n = backupNotification(databases.BackupFinished{DatabaseName: "main", Engine: "mysql", Succeeded: true, Trigger: "manual", SizeBytes: 1_400_000, OffSite: true}, "x")
	if n.Kind != domain.BackupSucceeded || n.Body != "A backup started by hand of mysql: 1.4 MB, kept on the server's disk and uploaded to S3." {
		t.Fatalf("got %+v", n)
	}
}
