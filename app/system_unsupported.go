//go:build !windows && !darwin && !linux && !(js && wasm)

package app

import (
	"os"

	"github.com/vivaui/viva/gfx"
)

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

func nativeShowFatalError(string, string) {}

func nativeMonospaceFamilies() []string { return nil }
func nativeFontFamilies() []string      { return nil }
func nativeFontWeights(string) []int    { return nil }

func nativeAppearance() Appearance { return AppearanceUnknown }

func nativeWatchAppearance() {}

func nativeSetAppearance(Appearance) {}

// nativeStartFileDrag is the Web-independent fallback: a target with neither a
// window the platform can drag from nor the Web's own drag has no way to take
// files out of one.
func nativeStartFileDrag([]string, FileDragEffect) (FileDragEffect, error) {
	return FileDragNone, ErrUnsupported
}
