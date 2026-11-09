package ui_test

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Box reads Rect after End, and stops naming its element at the next Begin.
func TestBoxIsValidAfterEndAndStaleAfterTheNextBegin(t *testing.T) {
	var view ui.View
	var box ui.Box
	view.Draw(benchCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *ui.Frame) {
		box = f.Row().Size(20, 20)
	})
	if !box.Valid() {
		t.Fatal("a Box is not valid after End")
	}
	_ = box.Rect()
	view.Draw(benchCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *ui.Frame) {
		f.Row().Size(20, 20) // the same node index is reused
	})
	if box.Valid() {
		t.Fatal("a Box from the last paint is valid after the next Begin")
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "after its paint") {
			t.Fatalf("recover() = %v, want a stale Box to panic", r)
		}
	}()
	_ = box.Rect()
}

// The generation fits the padding of the two words a Box was.
func TestBoxKeepsItsSize(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 && unsafe.Sizeof(ui.Box{}) != 16 {
		t.Fatalf("Box is %d bytes on 64-bit, want 16", unsafe.Sizeof(ui.Box{}))
	}
}
