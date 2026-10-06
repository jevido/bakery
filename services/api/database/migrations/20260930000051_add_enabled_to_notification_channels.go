package migrations

// M20260930000051AddEnabledToNotificationChannels lets an admin disable a
// Notification channel, as Coolify's Enable/Disable does. Existing channels
// stay enabled.
type M20260930000051AddEnabledToNotificationChannels struct{}

func (r *M20260930000051AddEnabledToNotificationChannels) Signature() string {
	return "20260930000051_add_enabled_to_notification_channels"
}

func (r *M20260930000051AddEnabledToNotificationChannels) Up() error {
	return sqls(`ALTER TABLE notification_channels ADD COLUMN enabled boolean NOT NULL DEFAULT true`)
}

func (r *M20260930000051AddEnabledToNotificationChannels) Down() error {
	return sqls(`ALTER TABLE notification_channels DROP COLUMN IF EXISTS enabled`)
}
