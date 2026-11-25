//go:build darwin || windows || linux

package app

import "runtime"

func nativeCapabilities() Capabilities {
	capabilities := CapabilityMultiWindow |
		CapabilityStatusItem |
		CapabilityFileDrop |
		CapabilityClipboardText |
		CapabilityClipboardTextAsync |
		CapabilityClipboardImage |
		CapabilityClipboardImageAsync |
		CapabilityClipboardImageWrite |
		CapabilityClipboardImageWriteAsync |
		CapabilityClipboardFiles |
		CapabilityClipboardHTML |
		CapabilityFileDialogs |
		CapabilityFileDialogsAsync |
		CapabilityDirectoryDialogs |
		CapabilitySecretStore |
		CapabilitySecretStoreAsync |
		CapabilityNotifications |
		CapabilitySystemSound |
		CapabilityFontEnumeration |
		CapabilityFontRegistration |
		CapabilityFontRegistrationAsync |
		CapabilityWebView |
		CapabilityCustomURLScheme |
		CapabilityOpenURL |
		CapabilityFullscreen |
		CapabilityWindowFrame |
		CapabilityWindowMinimize |
		CapabilityReduceMotion |
		CapabilityPreventSleep |
		CapabilitySystemProxy |
		capabilityFileDrag
	if runtime.GOOS == "darwin" {
		capabilities |= CapabilityNativeAppearance
	} else {
		capabilities |= CapabilityWindowMaximize
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		capabilities |= CapabilityTransparentWindow
	}
	return Capabilities(capabilities)
}
