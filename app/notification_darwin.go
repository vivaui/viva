//go:build darwin && !ios

package app

import (
	"github.com/vivaui/viva/internal/platform"
	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

func nativeShowNotification(notification Notification, events NotificationEvents) (func(), error) {
	failed := func(err error) {
		err = darwinNotificationError(err)
		if events.OnError != nil {
			events.OnError(err)
		} else {
			reportNotificationError(err)
		}
	}
	cancel, err := darwinplatform.ShowNotification(notification.Title, notification.Body, platform.NotificationEvents{Activated: events.OnActivate, Failed: failed})
	return cancel, darwinNotificationError(err)
}
