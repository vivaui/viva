//go:build windows

package windows

import "testing"

// TestCursorShapesIncludeHidden asks for every named shape, including the
// transparent one a surface that draws its own pointer asks for.
func TestCursorShapesIncludeHidden(t *testing.T) {
	for shape := uint8(0); shape <= cursorHidden; shape++ {
		handle, err := cursorHandle(shape)
		if err != nil {
			t.Fatalf("shape %d: %v", shape, err)
		}
		if handle == 0 {
			t.Fatalf("shape %d: no cursor handle", shape)
		}
	}
	if _, err := cursorHandle(cursorHidden + 1); err == nil {
		t.Fatal("a shape past the last one was accepted")
	}
}
