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
		capabilityFileDrag |
		capabilityClipboardFilePromise
	if runtime.GOOS == "darwin" {
		capabilities |= CapabilityNativeAppearance
	} else {
		capabilities |= CapabilityWindowMaximize
	}
	capabilities |= capabilityTransparentWindow | capabilityNotificationActivation | capabilityWindowWatermark
	return Capabilities(capabilities)
}
