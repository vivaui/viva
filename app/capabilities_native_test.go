//go:build darwin || windows || linux

package app

import "testing"

func TestDesktopCapabilitiesKeepSynchronousIntegrations(t *testing.T) {
	if !Supports(CapabilityMultiWindow) {
		t.Fatal("desktop reports no multi-window support")
	}
	if !Supports(CapabilityClipboardText) {
		t.Fatal("desktop reports no synchronous clipboard text")
	}
	if Supports(CapabilityNotificationPermission) {
		t.Fatal("desktop reports a browser-style notification permission request")
	}
}
