//go:build linux

package linux

import (
	"errors"
	"testing"
)

// TestPreventSleepWhenLogindIsThere holds a lock only where the machine has
// one. A build container or a session without logind has no system bus to ask,
// and skipping there hides nothing this code promises.
func TestPreventSleepWhenLogindIsThere(t *testing.T) {
	release, err := PreventSleep("viva test: sleep inhibition")
	if errors.Is(err, ErrSleepUnavailable) {
		t.Skipf("no logind inhibitor: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // releasing twice is not an error and does not close another lock
}
