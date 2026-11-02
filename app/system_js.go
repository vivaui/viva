//go:build js && wasm

package app

import (
	"errors"
	"fmt"

	webplatform "github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/ui"
)

func nativeReduceMotion() bool { return false }

// nativeTextEngine is the platform's text engine; nil keeps the reference metrics.
func nativeTextEngine() ui.TextEngine { return nil }

func nativePreferredLanguages() []string { return webplatform.PreferredLanguages() }

func nativeClipboardText() (string, error) {
	value, err := webplatform.ClipboardText()
	return value, webSystemError("read clipboard", err)
}

func nativeOnClipboardChange(notify func()) error {
	if notify == nil {
		return nil
	}
	return webSystemError("watch clipboard", webplatform.ErrUnavailable)
}

func nativeSetClipboardText(value string) error {
	return webSystemError("set clipboard", webplatform.SetClipboardText(value))
}

func nativeHasClipboardImage() bool { return false }

func nativeClipboardImagePNG() ([]byte, error) {
	return nil, webSystemError("read clipboard image", webplatform.ClipboardImagePNG())
}

func nativeSetCursor(cursor Cursor) error {
	return webSystemError("set cursor", webplatform.SetCursorShape(uint8(cursor)))
}

func nativeSetCursorImage(image *ui.CursorImage) error {
	return webSystemError("set cursor image", webplatform.SetCursorImage(image))
}

func nativeOpenFiles(OpenFileOptions) ([]string, error) {
	paths, err := webplatform.OpenFiles()
	return paths, webSystemError("open files", err)
}

func nativeOpenDirectory(OpenDirectoryOptions) (string, error) {
	return "", ErrUnsupported
}

func nativeSaveFile(SaveFileOptions) (string, error) {
	path, err := webplatform.SaveFile()
	return path, webSystemError("save file", err)
}

func nativeOpenURL(target string) error {
	return webSystemError("open external URL", webplatform.OpenURL(target))
}

func nativePlaySystemSound(sound SystemSound) error {
	return webSystemError("play system sound", webplatform.PlaySystemSound(uint8(sound)))
}

func nativeSendNotification(notification Notification) error {
	return webSystemError("send notification", webplatform.SendNotification(notification.Title, notification.Body))
}

func webSystemError(action string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, webplatform.ErrAsyncRequired):
		return fmt.Errorf("%s: %w: %v", action, ErrAsyncRequired, err)
	case errors.Is(err, webplatform.ErrUserGestureRequired):
		return fmt.Errorf("%s: %w: %v", action, ErrUserGestureRequired, err)
	case errors.Is(err, webplatform.ErrPermissionDenied):
		return fmt.Errorf("%s: %w: %v", action, ErrPermissionDenied, err)
	case errors.Is(err, webplatform.ErrUnsupported), errors.Is(err, webplatform.ErrUnavailable):
		return fmt.Errorf("%s: %w: %v", action, ErrUnsupported, err)
	default:
		return fmt.Errorf("app: %s: %w", action, err)
	}
}

// A browser has no store for credentials that is not the page's own storage.
func nativeStoreSecret(string, string, []byte) error  { return ErrSecretStoreUnavailable }
func nativeLoadSecret(string, string) ([]byte, error) { return nil, ErrSecretStoreUnavailable }
func nativeDeleteSecret(string, string) error         { return ErrSecretStoreUnavailable }

// A page cannot list the fonts of the machine it runs on.
func nativeEnableCrashDumps(string) (string, error) { return "", ErrUnsupported }

func nativeShowFatalError(string, string) {}

func nativeMonospaceFamilies() []string { return nil }
func nativeFontFamilies() []string      { return nil }

func nativeAppearance() Appearance { return appearanceOf(webplatform.DarkAppearance()) }

func nativeWatchAppearance() { webplatform.WatchAppearance() }

func nativeSetAppearance(Appearance) {}
