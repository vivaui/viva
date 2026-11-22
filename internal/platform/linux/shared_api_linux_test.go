package linux

import "testing"

// The libxcb library is opened once per process, and a machine
// without one is found once; every caller gets its own copy of the bound table,
// so wrapping one window's functions leaves another's alone.
func TestXCBTableIsLoadedOnce(t *testing.T) {
	x1, xErr1 := loadXCB()
	x2, xErr2 := loadXCB()
	if xErr1 != xErr2 || (xErr1 == nil && (x1.library != x2.library || x1 == x2)) {
		t.Fatalf("loadXCB gave %p, %v then %p, %v", x1, xErr1, x2, xErr2)
	}
	if xErr1 == nil {
		x1.connect = nil
		if x2.connect == nil {
			t.Fatal("changing one copy of the table changed another")
		}
	}
}
