//go:build windows

package windows

import "testing"

func resetClipboardWatch(t *testing.T) {
	t.Helper()
	original := clipboardSequence
	t.Cleanup(func() {
		clipboardSequence = original
		OnClipboardChange(nil)
	})
	OnClipboardChange(nil)
}

// Every window of the process hears each change, and the callback runs once for it: the
// clipboard's sequence number moves once per change.
func TestClipboardChangeIsReportedOncePerChangeWhateverWindowsHearIt(t *testing.T) {
	resetClipboardWatch(t)
	sequence := uint32(10)
	clipboardSequence = func() uint32 { return sequence }
	calls := 0
	if err := OnClipboardChange(func() { calls++ }); err != nil {
		t.Fatal(err)
	}

	notifyClipboardChange() // the sequence number has not moved since registering
	if calls != 0 {
		t.Fatalf("calls = %d before any change", calls)
	}
	sequence = 11
	notifyClipboardChange() // the first window to hear the change
	notifyClipboardChange() // a second window hears the same one
	if calls != 1 {
		t.Fatalf("calls = %d for one change heard twice, want 1", calls)
	}
	sequence = 12
	notifyClipboardChange()
	if calls != 2 {
		t.Fatalf("calls = %d after a second change, want 2", calls)
	}
}

// A process that may not read the sequence number (it reads 0) still gets the news, once for every
// message, rather than never.
func TestClipboardChangeWithoutASequenceNumberIsReportedEveryTime(t *testing.T) {
	resetClipboardWatch(t)
	clipboardSequence = func() uint32 { return 0 }
	calls := 0
	OnClipboardChange(func() { calls++ })
	notifyClipboardChange()
	notifyClipboardChange()
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

// Registering needs no window: the watch is kept for the windows that open later, and nil stops it.
func TestClipboardChangeMayBeRegisteredBeforeAnyWindowAndStops(t *testing.T) {
	resetClipboardWatch(t)
	sequence := uint32(1)
	clipboardSequence = func() uint32 { return sequence }
	calls := 0
	if err := OnClipboardChange(func() { calls++ }); err != nil {
		t.Fatalf("registering before a window opens: %v", err)
	}
	if clipboardWatch.Load() == nil {
		t.Fatal("the registration was not kept for the first window")
	}
	// A window that opens now joins the listener list; a window handle that is not real is refused
	// by the system and leaves nothing behind.
	if err := listenToClipboard(0xdead); err == nil {
		t.Fatal("a window that does not exist was added to the listener list")
	}
	if _, kept := clipboardListeners.Load(uintptr(0xdead)); kept {
		t.Fatal("a failed registration was remembered")
	}

	if err := OnClipboardChange(nil); err != nil {
		t.Fatal(err)
	}
	sequence = 2
	notifyClipboardChange()
	if calls != 0 {
		t.Fatalf("calls = %d after stopping", calls)
	}
	if err := listenToClipboard(0xdead); err != nil {
		t.Fatalf("a stopped watch still added a window: %v", err)
	}
}
