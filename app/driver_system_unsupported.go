//go:build !darwin && !windows && !linux && !(js && wasm)

package app

import "github.com/vivaui/viva/internal/platform"

func markNonNativeSystemUsed() {}

func driverClipboardText() (string, error, bool) { return "", nil, false }

func driverSetClipboardText(string) (error, bool) { return nil, false }

func driverSetRectangularClipboardText(string) (error, bool) { return nil, false }

func driverClipboardTextWithFormat() (string, bool, error, bool) { return "", false, nil, false }

func driverSetPrimarySelection(string) (error, bool) { return nil, false }

func driverPrimarySelection() (string, error, bool) { return "", nil, false }

func driverClipboardImage() (png []byte, err error, handled bool) { return nil, nil, false }

func driverClipboardFiles() (paths []string, err error, handled bool) { return nil, nil, false }

func driverSetClipboardFiles([]string) (error, bool) { return nil, false }

func driverStartFileDrag([]string, platform.DragEffect) (platform.DragEffect, error, bool) {
	return platform.DragEffectNone, nil, false
}

func driverSetClipboardHTML(string, string) (error, bool) { return nil, false }

func driverSendNotification(Notification) (error, bool) { return nil, false }

func driverOpenURL(string) (error, bool)    { return nil, false }
func driverOpenPath(string) (error, bool)   { return nil, false }
func driverRevealPath(string) (error, bool) { return nil, false }
func driverOpenWith(string, string) (error, bool) {
	return nil, false
}
func driverOpenInBrowser(string) (error, bool) { return nil, false }

func driverOpenFiles(OpenFileOptions) ([]string, error, bool) {
	return nil, nil, false
}

func driverOpenDirectory(OpenDirectoryOptions) (string, error, bool) {
	return "", nil, false
}

func driverSaveFile(SaveFileOptions) (string, error, bool) {
	return "", nil, false
}

func driverSaveFileWithCheckbox(SaveFileOptions, SaveFileCheckbox) (SaveFileSelection, error, bool) {
	return SaveFileSelection{}, nil, false
}

func driverStoreSecret(string, string, []byte) (error, bool) { return nil, false }

func driverLoadSecret(string, string) ([]byte, error, bool) { return nil, nil, false }

func driverDeleteSecret(string, string) (error, bool) { return nil, false }

func driverPreventSleep(string) (func(), error, bool) { return nil, nil, false }
