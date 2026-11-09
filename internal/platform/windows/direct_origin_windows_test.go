package windows

import (
	"image"
	"runtime"
	"syscall"
	"testing"
)

func TestDirectTextLayoutOriginByValue(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	format, err := r.services.formatFamily("Segoe UI", 16, 400)
	if err != nil {
		t.Fatal(err)
	}
	defer release(&format)
	units, _ := syscall.UTF16FromString("origin")
	var layout uintptr
	if err := hresult(r.services.createLayout(r.services.write, &units[0], uint32(len(units)-1), format, 200, 80, &layout)); err != nil {
		t.Fatal(err)
	}
	defer release(&layout)
	draw := func(x, y float32) image.Rectangle {
		scratch, err := r.opaque.bind(200, 80)
		if err != nil {
			t.Fatal(err)
		}
		fill(scratch, 200, 80, d2dColor{A: 1})
		r.opaque.begin(d2dColor{R: 1, G: 1, B: 1, A: 1}, d2dAntialiasGrayscale)
		r.opaque.draw(x, y, layout, 0)
		if err := r.opaque.end(); err != nil {
			t.Fatal(err)
		}
		bounds := image.Rectangle{}
		for row := 0; row < 80; row++ {
			for col := 0; col < 200; col++ {
				i := (row*scratch.Width + col) * 4
				if scratch.Pixels[i] != 0 || scratch.Pixels[i+1] != 0 || scratch.Pixels[i+2] != 0 {
					bounds = bounds.Union(image.Rect(col, row, col+1, row+1))
				}
			}
		}
		return bounds
	}
	first, second := draw(10, 8), draw(30, 24)
	if first.Empty() || second != first.Add(image.Pt(20, 16)) {
		t.Fatalf("origin did not translate native text: %v -> %v", first, second)
	}
}
