//go:build windows

package windows

import "testing"

// TestClipboardChangeCallbackRunsRegisteredHander checks the handoff the
// window procedure makes: the registered function runs, and clearing the
// registration stops the calls.
func TestClipboardChangeCallbackRunsRegisteredHander(t *testing.T) {
	calls := 0
	clipboardWatch.Store(nil)
	defer clipboardWatch.Store(nil)

	notifyClipboardChange() // nothing registered: no call, no panic
	handoff := func() { calls++ }
	clipboardWatch.Store(&handoff)
	notifyClipboardChange()
	notifyClipboardChange()
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}

	clipboardWatch.Store(nil)
	notifyClipboardChange() // cleared: no further calls
	if calls != 2 {
		t.Fatalf("calls after clear = %d, want 2", calls)
	}
}

// TestClipboardChangeWithoutWindowReportsIt: the registration tells the caller
// it cannot listen yet, rather than pretending the news will come.
func TestClipboardChangeWithoutWindowReportsIt(t *testing.T) {
	if active.Load() != nil {
		t.Skip("a window is running")
	}
	if err := OnClipboardChange(func() {}); err == nil {
		t.Fatal("listening without a window was accepted")
	}
}
