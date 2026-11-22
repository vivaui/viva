//go:build ios

package app

import (
	"fmt"
	"os"

	"github.com/vivaui/viva/gfx"
)

// Until the iOS backend provides them (ADR 0095), the platform's native hooks
// report that the system does not offer them: what a window and a UIKit host
// give, such as the clipboard, the cursor, dialogs and notifications, is added
// here one by one, and each moves out of this file as it lands.

func currentPlatform() Platform { return PlatformUnknown }

func nativeReduceMotion() bool                                 { return false }
func nativeTextEngine() gfx.TextEngine                         { return nil }
func nativePreferredLanguages() []string                       { return languagesFromEnvironment(os.Getenv) }
func nativeClipboardText() (string, error)                     { return "", ErrUnsupported }
func nativeSetClipboardText(string) error                      { return ErrUnsupported }
func nativeHasClipboardImage() bool                            { return false }
func nativeClipboardImagePNG() ([]byte, error)                 { return nil, ErrUnsupported }
func nativeSetClipboardImage([]byte) error                     { return ErrUnsupported }
func nativeClipboardFiles() ([]string, error)                  { return nil, ErrUnsupported }
func nativeSetClipboardFiles([]string) error                   { return ErrUnsupported }
func nativeSetClipboardHTML(html, alt string) error            { return ErrUnsupported }
func nativeOnClipboardChange(func()) error                     { return ErrUnsupported }
func nativeSetCursor(Cursor) error                             { return ErrUnsupported }
func nativeSetCursorImage(*gfx.CursorImage) error              { return ErrUnsupported }
func nativeOpenFiles(OpenFileOptions) ([]string, error)        { return nil, ErrUnsupported }
func nativeOpenDirectory(OpenDirectoryOptions) (string, error) { return "", ErrUnsupported }
func nativeSaveFile(SaveFileOptions) (string, error)           { return "", ErrUnsupported }
func nativeSaveFileWithCheckbox(SaveFileOptions, SaveFileCheckbox) (SaveFileSelection, error) {
	return SaveFileSelection{}, ErrUnsupported
}
func nativeOpenURL(string) error                      { return ErrUnsupported }
func nativeRevealPath(string) error                   { return ErrUnsupported }
func nativePlaySystemSound(SystemSound) error         { return ErrUnsupported }
func nativeSendNotification(Notification) error       { return ErrUnsupported }
func nativeStoreSecret(string, string, []byte) error  { return ErrSecretStoreUnavailable }
func nativeLoadSecret(string, string) ([]byte, error) { return nil, ErrSecretStoreUnavailable }
func nativeDeleteSecret(string, string) error         { return ErrSecretStoreUnavailable }

func nativeEnableCrashDumps(string) (string, error) { return "", ErrUnsupported }
func nativeShowFatalError(string, string)           {}

func nativeMonospaceFamilies() []string { return nil }
func nativeFontFamilies() []string      { return nil }
func nativeFontWeights(string) []int    { return nil }

func nativeAppearance() Appearance   { return AppearanceUnknown }
func nativeWatchAppearance()         {}
func nativeSetAppearance(Appearance) {}

func nativeStartFileDrag([]string, FileDragEffect) (FileDragEffect, error) {
	return FileDragNone, ErrUnsupported
}

func nativeFileIcon(string, int) ([]byte, error)                          { return nil, ErrUnsupported }
func nativeRegisterGlobalShortcut(GlobalShortcut, func()) (func(), error) { return nil, ErrUnsupported }
func nativePreventSleep(string) (func(), error)                           { return nil, ErrSleepPreventionUnavailable }
func nativeSchemeHandlerAvailable(string) (bool, error)                   { return false, ErrUnsupported }
func nativeOpenInBrowser(string) error                                    { return fmt.Errorf("%w: OpenPathInBrowser", ErrUnsupported) }
func nativeOpenWith(_, _ string) error                                    { return fmt.Errorf("%w: OpenWith", ErrUnsupported) }

func setReopenHandler(*Config, *Window) {}

// No window buttons: the system owns the window.
const controlsDrawnByViva = false

func platformTitlebarInsets() (leading, trailing float32) { return 0, 0 }
