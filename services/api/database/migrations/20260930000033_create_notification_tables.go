package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000033CreateNotificationTables holds the Notification channels
// (notifications), their settings encrypted as one JSON document, and the
// recent Deliveries of each.
type M20260930000033CreateNotificationTables struct{}

func (r *M20260930000033CreateNotificationTables) Signature() string {
	return "20260930000033_create_notification_tables"
}

func (r *M20260930000033CreateNotificationTables) Up() error {
	if !facades.Schema().HasTable("notification_channels") {
		if err := facades.Schema().Create("notification_channels", func(t schema.Blueprint) {
			t.ID()
			t.String("name", 63)
			t.String("kind", 16)
			t.Text("settings_encrypted")
			// Comma-separated Event kinds.
			t.Text("event_kinds")
			t.TimestampsTz()
			t.Unique("name")
		}); err != nil {
			return err
		}
	}
	if facades.Schema().HasTable("notification_deliveries") {
		return nil
	}
	return facades.Schema().Create("notification_deliveries", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("channel_id")
		t.String("event_kind", 32)
		t.Text("title")
		t.Text("body")
		t.Text("link")
		t.TimestampTz("happened_at", 6)
		t.String("status", 16)
		t.Integer("attempts").Default(0)
		t.Text("last_error")
		// Microseconds: at 0 digits a Delivery created at .6 s would only
		// be due a second later.
		t.TimestampTz("next_attempt_at", 6).Nullable()
		t.TimestampTz("sent_at", 6).Nullable()
		t.TimestampsTz()
		t.Index("channel_id", "id")
		t.Index("status", "next_attempt_at")
		t.Foreign("channel_id").References("id").On("notification_channels").CascadeOnDelete()
	})
}

func (r *M20260930000033CreateNotificationTables) Down() error {
	if err := facades.Schema().DropIfExists("notification_deliveries"); err != nil {
		return err
	}
	return facades.Schema().DropIfExists("notification_channels")
}
