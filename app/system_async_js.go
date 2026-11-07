//go:build js && wasm

package app

import webplatform "github.com/vivaui/viva/internal/platform/web"

func nativeClipboardTextAsync(done func(string, error)) error {
	return webSystemError("read clipboard", webplatform.ClipboardTextAsync(func(value string, err error) {
		done(value, webSystemError("read clipboard", err))
	}))
}

func nativeSetClipboardTextAsync(value string, done func(error)) error {
	return webSystemError("write clipboard", webplatform.SetClipboardTextAsync(value, func(err error) {
		done(webSystemError("write clipboard", err))
	}))
}

func nativeRequestNotificationPermission(done func(NotificationPermission, error)) error {
	return webSystemError("request notification permission", webplatform.RequestNotificationPermission(func(granted bool, err error) {
		if err != nil {
			done(NotificationPermissionDefault, webSystemError("request notification permission", err))
			return
		}
		if granted {
			done(NotificationPermissionGranted, nil)
			return
		}
		done(NotificationPermissionDenied, nil)
	}))
}
