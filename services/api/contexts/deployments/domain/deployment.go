// Package domain is the deployments model: a Deployment and the order its
// status moves in.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Status string

const (
	Queued   Status = "queued"
	Cloning  Status = "cloning"
	Building Status = "building"
	Starting Status = "starting"
	Finished Status = "finished"
	Failed   Status = "failed"
	// Cancelled is final, like Finished and Failed: the Owner stopped it.
	Cancelled Status = "cancelled"
)

// ActiveStatuses are the statuses of a Deployment still under way.
var ActiveStatuses = []Status{Queued, Cloning, Building, Starting}

// RunningStatuses are the active statuses past the queue. An Application has
// at most one queued and one running Deployment; a queued one waits until the
// Application has no running one.
var RunningStatuses = []Status{Cloning, Building, Starting}

var order = map[Status]int{Queued: 0, Cloning: 1, Building: 2, Starting: 3, Finished: 4}

// Active reports whether s is one of ActiveStatuses.
func (s Status) Active() bool {
	_, ok := order[s]
	return ok && s != Finished
}

// Trigger is what started a Deployment.
type Trigger string

const (
	TriggerManual   Trigger = "manual"
	TriggerWebhook  Trigger = "webhook"
	TriggerRollback Trigger = "rollback"
)

var (
	ErrAlreadyQueued = errors.New("a deployment of this application is already queued")
	// ErrCancelled is why a cancelled Deployment's work stopped.
	ErrCancelled = errors.New("deployment cancelled")
	// ErrNotRollbackTarget is a Rollback to a Deployment that did not
	// finish, or has no Image.
	ErrNotRollbackTarget = errors.New("only a finished deployment can be rolled back to")
)

type Deployment struct {
	ID            uint64
	ApplicationID uint64
	Status        Status
	Trigger       Trigger
	Branch        string
	CommitSHA     string
	CommitMessage string
	CommitAuthor  string
	// SourceImage is the reference with digest an image Deployment pulled.
	SourceImage string
	Image       string
	Container   string
	// RollbackOf is the Deployment whose Image a Rollback starts again.
	RollbackOf *uint64
	Error      string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// Advance moves the Deployment forward to status. Statuses only move
// forward, and only an active Deployment moves at all.
func (d *Deployment) Advance(to Status) error {
	if !d.Status.Active() {
		return fmt.Errorf("deployment %d is %s and cannot move to %s", d.ID, d.Status, to)
	}
	if to == Failed {
		return fmt.Errorf("use Fail to fail deployment %d", d.ID)
	}
	if order[to] <= order[d.Status] {
		return fmt.Errorf("deployment %d cannot move back from %s to %s", d.ID, d.Status, to)
	}
	d.Status = to
	return nil
}

// Fail ends an active Deployment with the reason.
func (d *Deployment) Fail(reason string) error {
	if !d.Status.Active() {
		return fmt.Errorf("deployment %d is %s and cannot fail", d.ID, d.Status)
	}
	d.Status, d.Error = Failed, reason
	return nil
}

// NewDeployment is a queued Deployment of the Application.
func NewDeployment(applicationID uint64, trigger Trigger) Deployment {
	return Deployment{ApplicationID: applicationID, Status: Queued, Trigger: trigger}
}

// NewRollback is a queued Deployment that starts of's Image again, showing
// of's branch and commit. Only a finished Deployment with an Image can be
// rolled back to.
func NewRollback(of Deployment) (Deployment, error) {
	if of.Status != Finished || of.Image == "" {
		return Deployment{}, ErrNotRollbackTarget
	}
	id := of.ID
	d := NewDeployment(of.ApplicationID, TriggerRollback)
	d.RollbackOf, d.Image = &id, of.Image
	d.Branch, d.CommitSHA, d.CommitMessage, d.CommitAuthor = of.Branch, of.CommitSHA, of.CommitMessage, of.CommitAuthor
	d.SourceImage = of.SourceImage
	return d, nil
}

// Cancel ends an active Deployment as cancelled.
func (d *Deployment) Cancel() error {
	if !d.Status.Active() {
		return fmt.Errorf("deployment %d is %s and cannot be cancelled", d.ID, d.Status)
	}
	d.Status, d.Error = Cancelled, ""
	return nil
}

// ContainerName names the Container a Deployment runs.
func ContainerName(applicationID, deploymentID uint64) string {
	return "bakery-app-" + strconv.FormatUint(applicationID, 10) + "-" + strconv.FormatUint(deploymentID, 10)
}

// VolumeName names the Volume behind an Application's Persistent storage.
// It holds no Deployment id: every Deployment mounts the same Volume.
func VolumeName(applicationID uint64, storage string) string {
	return "bakery-app-" + strconv.FormatUint(applicationID, 10) + "-" + storage
}

// ImageTag names the Image a Deployment builds.
func ImageTag(slug string, deploymentID uint64) string {
	return "localhost/bakery/" + slug + ":" + strconv.FormatUint(deploymentID, 10)
}

// KnownHost is a git host's SSH host keys, recorded on the first clone from
// it. Keys are known_hosts lines; Fingerprints their SHA256 fingerprints.
type KnownHost struct {
	ID           uint64
	Host         string
	Keys         string
	Fingerprints []string
	CreatedAt    time.Time
}

// Log streams.
const (
	StreamInfo = "info"
	StreamOut  = "out"
	StreamErr  = "err"
)

// LogLine is one line of a Deployment log.
type LogLine struct {
	ID     uint64
	Stream string
	Line   string
}
