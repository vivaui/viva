//go:build darwin

package app

import (
	"errors"
	"fmt"

	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
	"github.com/vivaui/viva/ui"
)

func nativeClipboardText() (string, error) { return darwinplatform.ClipboardText() }

func nativeHasClipboardImage() bool { return darwinplatform.HasClipboardImage() }

func nativeClipboardImagePNG() ([]byte, error) { return darwinplatform.ClipboardImagePNG() }

func nativeReduceMotion() bool { return darwinplatform.ReduceMotion() }

// nativeTextEngine is the platform's text engine; nil keeps the reference metrics.
func nativeTextEngine() ui.TextEngine { return darwinplatform.SharedTextEngine() }

func nativePreferredLanguages() []string { return darwinplatform.PreferredLanguages() }

func nativeOnClipboardChange(notify func()) error {
	return darwinplatform.OnClipboardChange(notify)
}

func nativeSetClipboardText(value string) error {
	return darwinplatform.SetClipboardText(value)
}

func nativeSetCursor(cursor Cursor) error {
	return darwinplatform.SetCursorShape(uint8(cursor))
}

func nativeSetCursorImage(image *ui.CursorImage) error {
	return darwinplatform.SetCursorImage(image)
}

func nativeOpenFiles(options OpenFileOptions) ([]string, error) {
	filters := make([]darwinplatform.FileFilter, len(options.Filters))
	for index, filter := range options.Filters {
		filters[index] = darwinplatform.FileFilter{Name: filter.Name, Pattern: filter.Pattern}
	}
	paths, canceled, err := darwinplatform.OpenFiles(darwinplatform.OpenFileOptions{
		Title: options.Title, Filters: filters, AllowMultiple: options.AllowMultiple,
	})
	if canceled {
		return nil, ErrCanceled
	}
	return paths, err
}

func nativeOpenDirectory(options OpenDirectoryOptions) (string, error) {
	path, canceled, err := darwinplatform.OpenDirectory(darwinplatform.OpenDirectoryOptions{Title: options.Title})
	if canceled {
		return "", ErrCanceled
	}
	return path, err
}

func nativeSaveFile(options SaveFileOptions) (string, error) {
	filters := make([]darwinplatform.FileFilter, len(options.Filters))
	for index, filter := range options.Filters {
		filters[index] = darwinplatform.FileFilter{Name: filter.Name, Pattern: filter.Pattern}
	}
	path, canceled, err := darwinplatform.SaveFile(darwinplatform.SaveFileOptions{
		Title: options.Title, SuggestedName: options.SuggestedName,
		DefaultExtension: options.DefaultExtension, Filters: filters,
	})
	if canceled {
		return "", ErrCanceled
	}
	return path, err
}

func nativeOpenURL(target string) error { return darwinplatform.OpenURL(target) }

func nativePlaySystemSound(sound SystemSound) error {
	return darwinplatform.PlaySystemSound(uint8(sound))
}

func nativeSendNotification(notification Notification) error {
	return darwinNotificationError(darwinplatform.SendNotification(notification.Title, notification.Body))
}

func darwinNotificationError(err error) error {
	if errors.Is(err, darwinplatform.ErrPermissionDenied) {
		return fmt.Errorf("send notification: %w", ErrPermissionDenied)
	}
	return err
}

func nativeStoreSecret(service, account string, secret []byte) error {
	return secretError(darwinplatform.StoreSecret(service, account, secret))
}

func nativeLoadSecret(service, account string) ([]byte, error) {
	secret, err := darwinplatform.LoadSecret(service, account)
	return secret, secretError(err)
}

func nativeDeleteSecret(service, account string) error {
	return secretError(darwinplatform.DeleteSecret(service, account))
}

func nativeEnableCrashDumps(string) (string, error) { return "", ErrUnsupported }

func nativeShowFatalError(title, message string) { darwinplatform.ShowFatalError(title, message) }

func nativeMonospaceFamilies() []string { return darwinplatform.MonospaceFamilies() }
func nativeFontFamilies() []string      { return darwinplatform.FontFamilies() }

func nativeAppearance() Appearance {
	if darwinplatform.DarkAppearance() {
		return AppearanceDark
	}
	return AppearanceLight
}

func nativeWatchAppearance() { darwinplatform.WatchAppearance() }

func nativeSetAppearance(appearance Appearance) {
	darwinplatform.SetAppearance(appearance == AppearanceLight, appearance == AppearanceDark)
}
