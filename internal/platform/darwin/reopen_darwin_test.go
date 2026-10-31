//go:build darwin

package darwin

import "testing"

func TestApplicationReopenDispatchRunsOutsideLockAndCanBeRemoved(t *testing.T) {
	application := &nativeApplication{}
	t.Cleanup(func() { application.SetReopenHandler(nil) })
	application.SetReopenHandler(nil)
	if deliverApplicationReopen() {
		t.Fatal("unregistered reopen suppressed the default window behavior")
	}
	count := 0
	application.SetReopenHandler(func() {
		count++
		application.SetReopenHandler(nil)
	})
	if !deliverApplicationReopen() || count != 1 || deliverApplicationReopen() {
		t.Fatal("application reopen handler was not dispatched and removed")
	}
}

func TestApplicationReopenPanicFallsBackToNativeHandling(t *testing.T) {
	application := &nativeApplication{}
	t.Cleanup(func() { application.SetReopenHandler(nil) })
	application.SetReopenHandler(func() { panic("reopen failure") })
	if deliverApplicationReopen() {
		t.Fatal("panicking application reopen handler was reported handled")
	}
}
