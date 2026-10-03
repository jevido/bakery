package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000042RenameNotificationEventKinds gives the Event kinds
// Coolify's names, in the channels' subscriptions (a comma list) and in the
// Deliveries already made.
type M20260930000042RenameNotificationEventKinds struct{}

var notificationEventKindRenames = [][2]string{
	{"deployment_failed", "deployment_failure"},
	{"deployment_succeeded", "deployment_success"},
	{"backup_failed", "backup_failure"},
	{"backup_succeeded", "backup_success"},
	{"disk_almost_full", "server_disk_usage"},
}

func (r *M20260930000042RenameNotificationEventKinds) Signature() string {
	return "20260930000042_rename_notification_event_kinds"
}

func (r *M20260930000042RenameNotificationEventKinds) Up() error {
	for _, p := range notificationEventKindRenames {
		if err := renameEventKind(p[0], p[1]); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000042RenameNotificationEventKinds) Down() error {
	for _, p := range notificationEventKindRenames {
		if err := renameEventKind(p[1], p[0]); err != nil {
			return err
		}
	}
	return nil
}

func renameEventKind(from, to string) error {
	// array_replace swaps whole elements only, so "backup_failed" never
	// matches inside a longer name.
	if err := facades.Schema().Sql(`UPDATE notification_channels SET event_kinds = array_to_string(array_replace(string_to_array(event_kinds, ','), '` + from + `', '` + to + `'), ',')`); err != nil {
		return err
	}
	return facades.Schema().Sql(`UPDATE notification_deliveries SET event_kind = '` + to + `' WHERE event_kind = '` + from + `'`)
}
