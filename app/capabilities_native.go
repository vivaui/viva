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
		CapabilityClipboardFiles |
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
		CapabilityReduceMotion
	if runtime.GOOS == "darwin" {
		capabilities |= CapabilityNativeAppearance
	} else {
		capabilities |= CapabilityWindowMaximize
	}
	return Capabilities(capabilities)
}
