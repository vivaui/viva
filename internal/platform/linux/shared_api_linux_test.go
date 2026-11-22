package linux

import "testing"

// The libxcb and libvulkan libraries are opened once per process, and a machine
// without one is found once; every caller gets its own copy of the bound table,
// so wrapping one window's functions leaves another's alone.
func TestFunctionTablesAreLoadedOnce(t *testing.T) {
	x1, xErr1 := loadXCB()
	x2, xErr2 := loadXCB()
	if xErr1 != xErr2 || (xErr1 == nil && (x1.library != x2.library || x1 == x2)) {
		t.Fatalf("loadXCB gave %p, %v then %p, %v", x1, xErr1, x2, xErr2)
	}
	v1, vErr1 := loadVulkan()
	v2, vErr2 := loadVulkan()
	if vErr1 != vErr2 || (vErr1 == nil && (v1.library != v2.library || v1 == v2)) {
		t.Fatalf("loadVulkan gave %p, %v then %p, %v", v1, vErr1, v2, vErr2)
	}
	if xErr1 == nil {
		x1.connect = nil
		if x2.connect == nil {
			t.Fatal("changing one copy of the table changed another")
		}
	}
}
