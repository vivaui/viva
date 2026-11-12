package app

import (
	"fmt"
	"strings"
)

// NotificationPermission is the browser's answer to a notification permission
// request. Desktop builds report Granted because the operating system owns the
// permission decision when a notification is delivered.
type NotificationPermission uint8

const (
	NotificationPermissionDefault NotificationPermission = iota
	NotificationPermissionGranted
	NotificationPermissionDenied
)

// ClipboardTextAsync starts reading the clipboard and calls done with the text
// once the platform delivers it. The call returns immediately when the request
// starts; an error means it could not start and done is not called. Browser
// permission prompts require calling it from an input handler.
func ClipboardTextAsync(done func(string, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil clipboard callback")
	}
	return nativeClipboardTextAsync(done)
}

// SetClipboardTextAsync starts writing value to the clipboard and calls done
// with nil once the platform accepted it. The call returns immediately when the
// request starts; an error means it could not start and done is not called.
func SetClipboardTextAsync(value string, done func(error)) error {
	if done == nil {
		return fmt.Errorf("app: nil clipboard callback")
	}
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("app: clipboard text contains NUL")
	}
	return nativeSetClipboardTextAsync(value, done)
}

// RequestNotificationPermission asks the browser for permission to show
// notifications. On desktop it calls done with Granted because the operating
// system asks for permission when a notification is delivered. done runs
// exactly once when the request starts successfully.
func RequestNotificationPermission(done func(NotificationPermission, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil notification callback")
	}
	return nativeRequestNotificationPermission(done)
}
