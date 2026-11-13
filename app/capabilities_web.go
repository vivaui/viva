//go:build js && wasm

package app

func nativeCapabilities() Capabilities {
	return Capabilities(
		CapabilityFileDrop |
			CapabilityClipboardTextAsync |
			CapabilityClipboardHTML |
			CapabilityClipboardImageAsync |
			CapabilityFileDialogsAsync |
			CapabilitySecretStoreAsync |
			CapabilityNotifications |
			CapabilityNotificationPermission |
			CapabilityFontRegistrationAsync |
			CapabilityOpenURL |
			CapabilityFullscreen |
			CapabilityPageVisibility |
			CapabilityReduceMotion,
	)
}
