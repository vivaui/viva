//go:build windows

package app

import (
	windowsplatform "github.com/vivaui/viva/internal/platform/windows"
	"github.com/vivaui/viva/ui"
)

func nativeClipboardText() (string, error) { return windowsplatform.ClipboardText() }

func nativeHasClipboardImage() bool { return windowsplatform.HasClipboardImage() }

func nativeClipboardImagePNG() ([]byte, error) { return windowsplatform.ClipboardImagePNG() }

func nativeReduceMotion() bool { return windowsplatform.ReduceMotion() }

// nativeTextEngine is the platform's text engine; nil keeps the reference metrics.
func nativeTextEngine() ui.TextEngine { return windowsplatform.TextEngine() }

func nativePreferredLanguages() []string { return windowsplatform.PreferredLanguages() }

func nativeSetClipboardText(value string) error {
	return windowsplatform.SetClipboardText(value)
}

func nativeOnClipboardChange(notify func()) error {
	return windowsplatform.OnClipboardChange(notify)
}
func nativeSetCursor(cursor Cursor) error {
	return windowsplatform.SetCursorShape(uint8(cursor))
}
func nativeOpenFiles(options OpenFileOptions) ([]string, error) {
	filters := make([]windowsplatform.FileFilter, len(options.Filters))
	for index, filter := range options.Filters {
		filters[index] = windowsplatform.FileFilter{Name: filter.Name, Pattern: filter.Pattern}
	}
	paths, canceled, err := windowsplatform.OpenFiles(windowsplatform.OpenFileOptions{
		Title: options.Title, Filters: filters, AllowMultiple: options.AllowMultiple,
	})
	if canceled {
		return nil, ErrCanceled
	}
	return paths, err
}

func nativeOpenDirectory(options OpenDirectoryOptions) (string, error) {
	path, canceled, err := windowsplatform.OpenDirectory(windowsplatform.OpenDirectoryOptions{Title: options.Title})
	if canceled {
		return "", ErrCanceled
	}
	return path, err
}
func nativeSaveFile(options SaveFileOptions) (string, error) {
	filters := make([]windowsplatform.FileFilter, len(options.Filters))
	for index, filter := range options.Filters {
		filters[index] = windowsplatform.FileFilter{Name: filter.Name, Pattern: filter.Pattern}
	}
	path, canceled, err := windowsplatform.SaveFile(windowsplatform.SaveFileOptions{
		Title: options.Title, SuggestedName: options.SuggestedName,
		DefaultExtension: options.DefaultExtension, Filters: filters,
	})
	if canceled {
		return "", ErrCanceled
	}
	return path, err
}
func nativeOpenURL(target string) error { return windowsplatform.OpenURL(target) }

func nativeOpenWith(executable, path string) error {
	return windowsplatform.OpenWith(executable, path)
}
func nativePlaySystemSound(sound SystemSound) error {
	return windowsplatform.PlaySystemSound(uint8(sound))
}
func nativeSendNotification(notification Notification) error {
	return windowsplatform.SendNotification(notification.Title, notification.Body)
}

func nativeStoreSecret(service, account string, secret []byte) error {
	return secretError(windowsplatform.StoreSecret(service, account, secret))
}

func nativeLoadSecret(service, account string) ([]byte, error) {
	secret, err := windowsplatform.LoadSecret(service, account)
	return secret, secretError(err)
}

func nativeDeleteSecret(service, account string) error {
	return secretError(windowsplatform.DeleteSecret(service, account))
}

func nativeEnableCrashDumps(relativeDir string) (string, error) {
	return windowsplatform.EnableCrashDumps(relativeDir)
}

func nativeShowFatalError(title, message string) { windowsplatform.ShowFatalError(title, message) }

func nativeMonospaceFamilies() []string { return windowsplatform.MonospaceFamilies() }
func nativeFontFamilies() []string      { return windowsplatform.FontFamilies() }

func nativeAppearance() Appearance { return appearanceOf(windowsplatform.DarkAppearance()) }

func nativeWatchAppearance() {} // the window procedure hears the settings change

func nativeSetAppearance(appearance Appearance) {
	windowsplatform.SetAppearance(appearance == AppearanceLight, appearance == AppearanceDark)
}

func nativeSetCursorImage(image *ui.CursorImage) error {
	return windowsplatform.SetCursorImage(image)
}
