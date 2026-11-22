//go:build darwin && !ios

package darwin

import "testing"

func TestSystemQuitDispatchRunsOutsideLockAndCanBeCanceled(t *testing.T) {
	application := &nativeApplication{}
	t.Cleanup(func() { application.SetQuitRequestHandler(nil) })
	application.SetQuitRequestHandler(nil)
	if deliverQuitRequest() {
		t.Fatal("handled an unregistered quit")
	}
	count := 0
	application.SetQuitRequestHandler(func() { count++; application.SetQuitRequestHandler(nil) })
	if !deliverQuitRequest() || count != 1 || deliverQuitRequest() {
		t.Fatal("quit handler lifecycle")
	}
}
