package domain

import "time"

// Notification is a message about one thing that happened, as a publisher
// hands it over. It is not stored on its own; its Deliveries are.
type Notification struct {
	Kind  EventKind
	Title string
	Body  string
	// Link points into the dashboard; may be empty.
	Link string
	At   time.Time
}

// Alarming says whether the Event kind is something going wrong, which
// channels that can mark it (ntfy's tags) do.
func (k EventKind) Alarming() bool {
	switch k {
	case DeploymentFailed, BackupFailed, ServerUnreachable, DiskAlmostFull:
		return true
	}
	return false
}
