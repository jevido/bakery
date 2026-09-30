package domain

import (
	"testing"
	"time"
)

func TestDeliveryRetriesThenFails(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	d := NewDelivery(1, Notification{Kind: BackupFailed}, now)
	if d.Status != Pending || !d.NextAttemptAt.Equal(now) {
		t.Fatalf("new %+v", d)
	}
	d.Fail("refused", now)
	if d.Status != Pending || d.Attempts != 1 || !d.NextAttemptAt.Equal(now.Add(10*time.Second)) {
		t.Fatalf("after 1: %+v", d)
	}
	d.Fail("refused", now)
	if d.Status != Pending || d.Attempts != 2 || !d.NextAttemptAt.Equal(now.Add(60*time.Second)) {
		t.Fatalf("after 2: %+v", d)
	}
	d.Fail("still refused", now)
	if d.Status != Failed || d.Attempts != 3 || d.LastError != "still refused" || !d.NextAttemptAt.IsZero() {
		t.Fatalf("after 3: %+v", d)
	}
	d.Succeed(now)
	if d.Status != Failed || d.Attempts != 3 {
		t.Fatalf("a failed Delivery changed: %+v", d)
	}
}

func TestDeliverySucceedsOnRetry(t *testing.T) {
	now := time.Now()
	d := NewDelivery(1, Notification{}, now)
	d.Fail("timeout", now)
	d.Succeed(now.Add(time.Minute))
	if d.Status != Sent || d.Attempts != 2 || d.LastError != "" || !d.SentAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("%+v", d)
	}
}
