package app

import (
	"github.com/vivaui/viva/internal/platform"
	windowsplatform "github.com/vivaui/viva/internal/platform/windows"
)

func nativeShowNotification(notification Notification, events NotificationEvents) (func(), error) {
	return windowsplatform.ShowNotification(notification.Title, notification.Body, platform.NotificationEvents{Activated: events.OnActivate, Failed: func(err error) {
		if events.OnError != nil {
			events.OnError(err)
		} else {
			reportNotificationError(err)
		}
	}})
}
