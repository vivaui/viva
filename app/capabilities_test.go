package app

import "testing"

func TestCurrentPlatformIsKnown(t *testing.T) {
	if CurrentPlatform() == PlatformUnknown {
		t.Fatal("CurrentPlatform returned PlatformUnknown")
	}
}

func TestSupportsLooksAtTheCurrentBuild(t *testing.T) {
	if !Supports(CapabilityOpenURL) {
		t.Fatal("OpenURL is not supported")
	}
	if !Supports(CapabilityClipboardTextAsync) {
		t.Fatal("ClipboardTextAsync is not supported")
	}
	if Supports(CapabilityMultiWindow) != Supports(CapabilityMultiWindow) {
		t.Fatal("Supports changed between calls")
	}
}
