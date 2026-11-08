//go:build js && wasm

package app

import "testing"

func TestWebCapabilitiesExposeAsyncNotSynchronousSystemAPIs(t *testing.T) {
	if Supports(CapabilityClipboardText) {
		t.Fatal("the browser reports synchronous clipboard text support")
	}
	if !Supports(CapabilityClipboardTextAsync) {
		t.Fatal("the browser reports no asynchronous clipboard text support")
	}
	if Supports(CapabilityMultiWindow) {
		t.Fatal("the browser reports multi-window support")
	}
	if !Supports(CapabilityPageVisibility) {
		t.Fatal("the browser reports no page visibility")
	}
	if !Supports(CapabilityNotificationPermission) {
		t.Fatal("the browser reports no notification permission request")
	}
}
