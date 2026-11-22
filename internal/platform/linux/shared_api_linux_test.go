package linux

import "testing"

// The libxcb and libvulkan function tables are the process's: every window, the
// clipboard and the desktop settings share the one loaded first, and a machine
// without the library is found once.
func TestFunctionTablesAreLoadedOnce(t *testing.T) {
	x1, xErr1 := loadXCB()
	x2, xErr2 := loadXCB()
	if x1 != x2 || xErr1 != xErr2 {
		t.Fatalf("loadXCB gave %p, %v then %p, %v", x1, xErr1, x2, xErr2)
	}
	v1, vErr1 := loadVulkan()
	v2, vErr2 := loadVulkan()
	if v1 != v2 || vErr1 != vErr2 {
		t.Fatalf("loadVulkan gave %p, %v then %p, %v", v1, vErr1, v2, vErr2)
	}
}
