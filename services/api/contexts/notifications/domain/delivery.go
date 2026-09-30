package domain

import "time"

// DeliveryStatus is where a Delivery stands.
type DeliveryStatus string

const (
	Pending DeliveryStatus = "pending"
	Sent    DeliveryStatus = "sent"
	Failed  DeliveryStatus = "failed"
)

// MaxAttempts is how often a Delivery is tried before it fails for good.
const MaxAttempts = 3

// KeptDeliveries is how many Deliveries of each channel are kept.
const KeptDeliveries = 50

// retryAfter is the wait after the n-th failed attempt (1-based).
var retryAfter = []time.Duration{10 * time.Second, 60 * time.Second}

// Delivery is one Notification sent to one Notification channel.
type Delivery struct {
	ID           uint64
	ChannelID    uint64
	Notification Notification
	Status       DeliveryStatus
	Attempts     int
	LastError    string
	// NextAttemptAt is when a pending Delivery is due.
	NextAttemptAt time.Time
	CreatedAt     time.Time
	SentAt        time.Time
}

// NewDelivery is n to the channel, due at once.
func NewDelivery(channelID uint64, n Notification, now time.Time) Delivery {
	return Delivery{ChannelID: channelID, Notification: n, Status: Pending, NextAttemptAt: now, CreatedAt: now}
}

// Succeed records an attempt that got through.
func (d *Delivery) Succeed(now time.Time) {
	if d.Status != Pending {
		return
	}
	d.Attempts++
	d.Status, d.LastError, d.SentAt, d.NextAttemptAt = Sent, "", now, time.Time{}
}

// Fail records an attempt that did not: it is retried later, or failed for
// good after MaxAttempts.
func (d *Delivery) Fail(reason string, now time.Time) {
	if d.Status != Pending {
		return
	}
	d.Attempts++
	d.LastError = reason
	if d.Attempts >= MaxAttempts {
		d.Status, d.NextAttemptAt = Failed, time.Time{}
		return
	}
	d.NextAttemptAt = now.Add(retryAfter[d.Attempts-1])
}

// FailOnce records a failed attempt that is not retried: a Test
// notification, which the admin is waiting for.
func (d *Delivery) FailOnce(reason string, now time.Time) {
	if d.Status != Pending {
		return
	}
	d.Attempts++
	d.Status, d.LastError, d.NextAttemptAt = Failed, reason, time.Time{}
}
