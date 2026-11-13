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

// The closed-hand cursor this package draws itself is made once and shared, as
// the stock cursors are: asking twice must not grow a second one.
func TestGrabbingCursorIsMadeOnce(t *testing.T) {
	first, err := grabbingCursorHandle()
	if err != nil {
		t.Fatal(err)
	}
	second, err := grabbingCursorHandle()
	if err != nil {
		t.Fatal(err)
	}
	if first == 0 || first != second {
		t.Fatalf("grabbing cursor handles = %d then %d, want one shared handle", first, second)
	}
	// closeGrabbingCursor destroys the handle; a later call makes a fresh one.
	closeGrabbingCursor()
	third, err := grabbingCursorHandle()
	if err != nil {
		t.Fatal(err)
	}
	if third == 0 || third == first {
		t.Fatalf("re-made grabbing cursor = %d, want a fresh handle after the close", third)
	}
	closeGrabbingCursor()
}
