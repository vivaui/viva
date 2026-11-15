//go:build darwin

package darwin

import "testing"

func TestPreventSleepHoldsAnAssertion(t *testing.T) {
	release, err := PreventSleep("viva test: sleep inhibition")
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
}
