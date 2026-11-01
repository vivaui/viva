package platform

import "testing"

// Neighbouring cells of a block element meet on one pixel edge at any pitch, and every
// character of the range has a shape that stays inside its cell.
func TestBlockElementsMeetWithoutSeams(t *testing.T) {
	full, ok := BlockElementOf("█")
	if !ok || full.Count != 1 {
		t.Fatalf("full block = %+v, %v", full, ok)
	}
	for _, pitch := range []float32{7.147, 8.797, 10.996, 13.75, 22} {
		var previousRight int
		for cell := 0; cell < 12; cell++ {
			x0, _, x1, _ := full.Rects[0].DeviceRect(float32(cell)*pitch, pitch, 19)
			if cell > 0 && x0 != previousRight {
				t.Fatalf("pitch %v: cell %d starts at %d, the cell before it ends at %d", pitch, cell, x0, previousRight)
			}
			previousRight = x1
		}
	}
	for r := rune(0x2580); r <= 0x259F; r++ {
		element, ok := BlockElementOf(string(r))
		if !ok || element.Count == 0 || element.Count > 2 || element.Shade == 0 {
			t.Errorf("%U: %+v, %v", r, element, ok)
		}
		for _, rect := range element.Rects[:element.Count] {
			if rect.X0 >= rect.X1 || rect.Y0 >= rect.Y1 || rect.X1 > 8 || rect.Y1 > 8 {
				t.Errorf("%U: rectangle %+v is empty or outside the cell", r, rect)
			}
		}
	}
	for _, text := range []string{"", "a", "▀▀", "▀́", string(rune(0x259F + 1)), string(rune(0x257F))} {
		if _, ok := BlockElementOf(text); ok {
			t.Errorf("%q is not one block element", text)
		}
	}
	if upper, _ := BlockElementOf("▀"); upper.Rects[0] != (BlockRect{0, 0, 8, 4}) {
		t.Errorf("upper half block = %+v", upper.Rects[0])
	}
}
