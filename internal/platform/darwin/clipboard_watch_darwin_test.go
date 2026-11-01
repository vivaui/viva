//go:build darwin

package darwin

import (
	"sync/atomic"
	"testing"
	"time"
)

// The pasteboard has no change notification, so the news comes from a timer that exists only
// while someone wants it. A change made after registering is reported, and after OnClipboardChange(nil)
// it is not.
func TestClipboardChangeIsReportedUntilStopped(t *testing.T) {
	var calls atomic.Int32
	if err := OnClipboardChange(func() { calls.Add(1) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { OnClipboardChange(nil) })

	if err := SetClipboardText("viva clipboard watch 1"); err != nil {
		t.Skipf("no pasteboard to change: %v", err)
	}
	waitFor(t, func() bool { return calls.Load() >= 1 }, "a change made after registering was not reported")

	if err := OnClipboardChange(nil); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	if err := SetClipboardText("viva clipboard watch 2"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(700 * time.Millisecond) // more than two polling intervals
	if after := calls.Load(); after != before {
		t.Fatalf("%d calls after stopping, want none", after-before)
	}
}

func waitFor(t *testing.T, done func() bool, failure string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal(failure)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
