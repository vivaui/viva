//go:build darwin

package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

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

func TestApplicationReopenPanicReachesTheFaultHandler(t *testing.T) {
	var got platform.Fault
	platform.SetFaultHandler(func(fault platform.Fault) { got = fault })
	defer platform.SetFaultHandler(nil)
	application := &nativeApplication{}
	t.Cleanup(func() { application.SetReopenHandler(nil) })
	application.SetReopenHandler(func() { panic("reopen failure") })
	deliverApplicationReopen()
	if got.Where != "darwin: application reopen" || got.Message != "reopen failure" || len(got.Stack) == 0 {
		t.Fatalf("fault = %+v", got)
	}
}
