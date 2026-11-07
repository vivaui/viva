//go:build !(js && wasm)

package app

func nativeClipboardTextAsync(done func(string, error)) error {
	value, err := ClipboardText()
	done(value, err)
	return nil
}

func nativeSetClipboardTextAsync(value string, done func(error)) error {
	done(SetClipboardText(value))
	return nil
}

func nativeRequestNotificationPermission(done func(NotificationPermission, error)) error {
	done(NotificationPermissionGranted, nil)
	return nil
}
