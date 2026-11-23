//go:build linux

package vulkan

import (
	"testing"
	"time"
)

// The probe runs before a window with a drawn frame is created, so it is on the
// way to the first frame; it must not be slow.
func TestSoftwareProbeIsQuickAndStable(t *testing.T) {
	start := time.Now()
	first := DeviceIsSoftware()
	took := time.Since(start)
	t.Logf("software device: %v, probe took %v", first, took)
	if took > 2*time.Second {
		t.Fatalf("the probe took %v", took)
	}
	if DeviceIsSoftware() != first {
		t.Fatal("the answer changed")
	}
}
