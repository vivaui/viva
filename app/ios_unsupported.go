//go:build ios

package app

import "os"

// What a phone has no use for, and Android answers with its common fallbacks:
// an iPhone has no file icons for the application to ask for, and no reopening.

func nativeFileIcon(string, int) ([]byte, error) { return nil, ErrUnsupported }
func setReopenHandler(*Config, *Window)          {}

// What the system answers an iPhone application with, until its backend does:

func nativeClipboardText() (string, error) { return "", ErrUnsupported }
func nativeSetClipboardText(string) error  { return ErrUnsupported }
func nativeOpenURL(string) error           { return ErrUnsupported }
func nativePreferredLanguages() []string   { return languagesFromEnvironment(os.Getenv) }
func nativeReduceMotion() bool             { return false }
func nativeAppearance() Appearance         { return AppearanceUnknown }
func nativeWatchAppearance()               {}
