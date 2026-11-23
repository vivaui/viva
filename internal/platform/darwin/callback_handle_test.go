//go:build darwin && !ios

package darwin

import "testing"

func TestAnAlreadyReleasedCallbackHandleIsIgnored(t *testing.T) {
	if _, ok := runStateFor(0); ok {
		t.Fatal("a zero cgo handle resolved to a run state")
	}
	vivaFocusLost(0, nil) // must not panic
}
