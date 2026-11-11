package ui_test

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/ui"
)

// A Box reads Rect after End, and stops naming its element at the next paint.
func TestBoxIsValidAfterEndAndStaleAfterTheNextBegin(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{}), benchCanvas()
	var box ui.Box
	drawPage(page, canvas, func(c ui.C) {
		box = ui.Row(c).Size(20, 20)
	})
	if !box.Valid() {
		t.Fatal("a Box is not valid after End")
	}
	_ = box.Rect()
	drawPage(page, canvas, func(c ui.C) {
		ui.Row(c).Size(20, 20) // the same node index is reused
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
