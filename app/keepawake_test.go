package app

import (
	"strings"
	"testing"
)

func TestPreventSleepValidatesItsReason(t *testing.T) {
	for _, reason := range []string{"", "   ", "\t", "a\x00b", "a\nb", "a\rb", strings.Repeat("x", maxSleepReasonLength+1)} {
		if _, err := PreventSleep(reason); err == nil {
			t.Fatalf("PreventSleep(%q) was accepted", reason)
		}
	}
}
