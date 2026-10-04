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

func TestNewRollback(t *testing.T) {
	of := Deployment{ID: 7, ApplicationID: 3, Status: Finished, Image: "localhost/bakery/x:7", Branch: "main", CommitSHA: "abc", CommitMessage: "Fix", CommitAuthor: "Jane"}
	d, err := NewRollback(of)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != Queued || d.Trigger != TriggerRollback || d.RollbackOf == nil || *d.RollbackOf != 7 || d.Image != of.Image || d.ApplicationID != 3 || d.CommitMessage != "Fix" {
		t.Fatalf("%+v", d)
	}
	for _, bad := range []Deployment{{ID: 1, Status: Failed, Image: "i"}, {ID: 1, Status: Building, Image: "i"}, {ID: 1, Status: Finished}} {
		if _, err := NewRollback(bad); err == nil {
			t.Errorf("rollback to %+v allowed", bad)
		}
	}
	// A Rollback skips cloning and building.
	d.Status = Cloning
	if err := d.Advance(Starting); err != nil {
		t.Fatal(err)
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

func TestNewRestart(t *testing.T) {
	if _, err := NewRestart(nil); err != ErrNothingToRestart {
		t.Fatalf("nil: %v", err)
	}
	if _, err := NewRestart(&Deployment{ID: 3, Status: Failed, Image: "i"}); err != ErrNothingToRestart {
		t.Fatalf("failed one: %v", err)
	}
	d, err := NewRestart(&Deployment{ID: 3, ApplicationID: 1, Status: Finished, Image: "i", ServerID: 2, CommitSHA: "abc"})
	if err != nil || d.Trigger != TriggerRestart || *d.RollbackOf != 3 || d.Image != "i" || d.ServerID != 2 || d.CommitSHA != "abc" || d.Status != Queued {
		t.Fatalf("restart %+v %v", d, err)
	}
}

func TestStatusOf(t *testing.T) {
	cases := []struct {
		states []ContainerState
		health Health
		want   ApplicationStatus
	}{
		{nil, UnknownHealth, StatusExited},
		{[]ContainerState{"exited"}, UnknownHealth, StatusExited},
		{[]ContainerState{"running"}, Healthy, "running:healthy"},
		{[]ContainerState{"running"}, UnknownHealth, "running:unknown"},
		{[]ContainerState{"running", "created"}, Unhealthy, "running:unhealthy"},
		{[]ContainerState{"running", "exited"}, Healthy, StatusDegraded},
		{[]ContainerState{"restarting"}, UnknownHealth, StatusRestarting},
	}
	for _, c := range cases {
		if got := StatusOf(c.states, c.health); got != c.want {
			t.Errorf("%v %s: %s, want %s", c.states, c.health, got, c.want)
		}
	}
	if !ApplicationStatus("running:unknown").Running() || StatusExited.Running() {
		t.Fatal("Running")
	}
}
