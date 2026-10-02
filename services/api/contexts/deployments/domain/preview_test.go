package domain

import (
	"errors"
	"testing"
	"time"
)

func TestPreviewClose(t *testing.T) {
	p := Preview{Number: 7, State: PreviewOpen}
	now := time.Now()
	if err := p.Close(now); err != nil || p.State != PreviewClosed || p.ClosedAt == nil {
		t.Fatalf("close: %v %+v", err, p)
	}
	if err := p.Close(now); !errors.Is(err, ErrPreviewClosed) {
		t.Fatalf("closing twice: %v", err)
	}
	p.Reopen()
	if p.State != PreviewOpen || p.ClosedAt != nil {
		t.Fatalf("reopen: %+v", p)
	}
}

func TestPreviewNames(t *testing.T) {
	if got := PreviewDomain(7, "app.localhost"); got != "pr-7.app.localhost" {
		t.Errorf("domain: %s", got)
	}
	if got := PreviewContainerName(3, 7, 42); got != "bakery-app-3-pr7-42" {
		t.Errorf("container: %s", got)
	}
	if got := PreviewVolumeName(3, 7, "data"); got != "bakery-app-3-pr7-data" {
		t.Errorf("volume: %s", got)
	}
	d := NewPreviewDeployment(3, 7, TriggerWebhook)
	d.ID = 42
	if got := DeploymentContainerName(d); got != "bakery-app-3-pr7-42" {
		t.Errorf("deployment container: %s", got)
	}
	d.Preview = 0
	if got := DeploymentContainerName(d); got != ContainerName(3, 42) {
		t.Errorf("own container: %s", got)
	}
}

func TestPreviewDeploymentIsNoRollbackTarget(t *testing.T) {
	d := NewPreviewDeployment(3, 7, TriggerWebhook)
	d.ID, d.Status, d.Image = 42, Finished, "localhost/bakery/app:42"
	if _, err := NewRollback(d); !errors.Is(err, ErrPreviewRollback) {
		t.Fatalf("rollback of a preview deployment: %v", err)
	}
}
