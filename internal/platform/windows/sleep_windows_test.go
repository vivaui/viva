//go:build windows

package windows

import "testing"

func TestPreventSleepHoldsAndReleasesAPowerRequest(t *testing.T) {
	release, err := PreventSleep("viva test: sleep inhibition")
	if err != nil {
		t.Skipf("this session cannot hold a power request: %v", err)
	}
	release()
	release()
}

func TestPreventSleepRejectsANulInTheReason(t *testing.T) {
	if _, err := PreventSleep("a\x00b"); err == nil {
		t.Fatal("a reason with a NUL was accepted")
	}
}
