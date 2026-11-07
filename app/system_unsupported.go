//go:build !windows && !darwin && !linux && !(js && wasm)

package app

import (
	"os"

	"github.com/vivaui/viva/ui"
)

func nativeReduceMotion() bool                                 { return false }
func nativeTextEngine() ui.TextEngine                          { return nil }
func nativePreferredLanguages() []string                       { return languagesFromEnvironment(os.Getenv) }
func nativeClipboardText() (string, error)                     { return "", ErrUnsupported }
func nativeSetClipboardText(string) error                      { return ErrUnsupported }
func nativeHasClipboardImage() bool                            { return false }
func nativeClipboardImagePNG() ([]byte, error)                 { return nil, ErrUnsupported }
func nativeClipboardFiles() ([]string, error)                  { return nil, ErrUnsupported }
func nativeSetClipboardFiles([]string) error                   { return ErrUnsupported }
func nativeOnClipboardChange(func()) error                     { return ErrUnsupported }
func nativeSetCursor(Cursor) error                             { return ErrUnsupported }
func nativeSetCursorImage(*ui.CursorImage) error               { return ErrUnsupported }
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

func nativeShowFatalError(string, string) {}

func nativeMonospaceFamilies() []string { return nil }
func nativeFontFamilies() []string      { return nil }

func nativeAppearance() Appearance { return AppearanceUnknown }

func nativeWatchAppearance() {}

func nativeSetAppearance(Appearance) {}
