package app

// Capability names a platform integration an application may use. Capabilities
// are static for a build target; a runtime permission may still refuse an
// individual call.
type Capability uint64

const (
	CapabilityMultiWindow Capability = 1 << iota
	CapabilityStatusItem
	CapabilityFileDrop
	CapabilityClipboardText
	CapabilityClipboardTextAsync
	CapabilityClipboardImage
	CapabilityClipboardImageAsync
	CapabilityClipboardFiles
	CapabilityFileDialogs
	CapabilityFileDialogsAsync
	CapabilityDirectoryDialogs
	CapabilitySecretStore
	CapabilityNotifications
	CapabilityNotificationPermission
	CapabilitySystemSound
	CapabilityFontEnumeration
	CapabilityFontRegistration
	CapabilityWebView
	CapabilityCustomURLScheme
	CapabilityOpenURL
	CapabilityFullscreen
	CapabilityWindowFrame
	CapabilityWindowMinimize
	CapabilityWindowMaximize
	CapabilityPageVisibility
	CapabilityNativeAppearance
	CapabilityReduceMotion
	CapabilitySecretStoreAsync
	CapabilityFontRegistrationAsync
)

// Capabilities is a set of platform integrations.
type Capabilities uint64

// Has reports whether capability is in the set.
func (capabilities Capabilities) Has(capability Capability) bool {
	return capabilities&Capabilities(capability) != 0
}

// SystemCapabilities reports the integrations this build target provides.
func SystemCapabilities() Capabilities { return nativeCapabilities() }

// Supports reports whether this build target provides capability.
func Supports(capability Capability) bool { return SystemCapabilities().Has(capability) }
