package domain

import "testing"

func TestAdvance(t *testing.T) {
	d := Deployment{ID: 1, Status: Queued}
	for _, s := range []Status{Cloning, Building, Starting, Finished} {
		if err := d.Advance(s); err != nil {
			t.Fatalf("advance to %s: %v", s, err)
		}
	}
	if err := d.Fail("late"); err == nil {
		t.Fatal("a finished deployment must not fail")
	}

	d = Deployment{ID: 2, Status: Building}
	if err := d.Advance(Cloning); err == nil {
		t.Fatal("must not move back")
	}
	if err := d.Fail("boom"); err != nil || d.Status != Failed || d.Error != "boom" {
		t.Fatalf("fail: %v %+v", err, d)
	}
	if err := d.Advance(Starting); err == nil {
		t.Fatal("a failed deployment must not move")
	}
}

func TestCancel(t *testing.T) {
	for _, s := range ActiveStatuses {
		d := Deployment{ID: 1, Status: s}
		if err := d.Cancel(); err != nil || d.Status != Cancelled {
			t.Fatalf("cancel from %s: %v %s", s, err, d.Status)
		}
		if d.Status.Active() {
			t.Fatal("cancelled must not be active")
		}
		if err := d.Advance(Finished); err == nil {
			t.Fatal("a cancelled deployment must not move")
		}
	}
	for _, s := range []Status{Finished, Failed, Cancelled} {
		d := Deployment{ID: 1, Status: s}
		if err := d.Cancel(); err == nil {
			t.Fatalf("cancelled a %s deployment", s)
		}
	}
}

func TestNames(t *testing.T) {
	if got := ContainerName(3, 17); got != "bakery-app-3-17" {
		t.Fatal(got)
	}
	if got := ImageTag("whoami", 17); got != "localhost/bakery/whoami:17" {
		t.Fatal(got)
	}
}
