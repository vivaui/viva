//go:build (!darwin && !windows) || ios

package app

func nativeShowNotification(Notification, NotificationEvents) (func(), error) {
	return nil, ErrUnsupported
}
