//go:build linux

package vulkan

import "testing"

// The libvulkan library is opened once per process, and a machine without one is
// found once; every caller gets its own copy of the bound table, so wrapping one
// window's functions leaves another's alone.
func TestVulkanTableIsLoadedOnce(t *testing.T) {
	v1, vErr1 := loadVulkan()
	v2, vErr2 := loadVulkan()
	if vErr1 != vErr2 || (vErr1 == nil && (v1.library != v2.library || v1 == v2)) {
		t.Fatalf("loadVulkan gave %p, %v then %p, %v", v1, vErr1, v2, vErr2)
	}
}
