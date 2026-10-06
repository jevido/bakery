package domain

import "time"

// Notification is a message about one thing that happened, as a publisher
// hands it over. It is not stored on its own; its Deliveries are.
type Notification struct {
	// GuildID is the Guild whose channels get it; 0 is every Guild's (the
	// Local server's health concerns them all).
	GuildID uint64
	Kind    EventKind
	Title   string
	Body    string
	// Link points into the dashboard; may be empty.
	Link string
	At   time.Time
	// Test marks a Test notification: like Coolify's, it mentions no one
	// and goes to no Telegram forum topic. Not stored; a Test is never
	// retried.
	Test bool
}

// Alarming says whether the Event kind is something going wrong, which
// channels that can mark it (ntfy's tags) do.
func (k EventKind) Alarming() bool {
	switch k {
	case DeploymentFailure, BackupFailure, ServerUnreachable, ServerDiskUsage:
		return true
	}
	return false
}
