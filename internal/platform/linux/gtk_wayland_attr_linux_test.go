//go:build linux && vivagtk

package linux

import (
	"testing"
	"unsafe"
)

func TestGTKWaylandWindowAttrLP64Layout(t *testing.T) {
	var attr gdkWindowAttr
	if size := unsafe.Sizeof(attr); size != 80 {
		t.Fatalf("GDK window attributes size=%d, want 80", size)
	}
	for _, field := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"Title", unsafe.Offsetof(attr.Title), 0},
		{"EventMask", unsafe.Offsetof(attr.EventMask), 8},
		{"X", unsafe.Offsetof(attr.X), 12},
		{"Y", unsafe.Offsetof(attr.Y), 16},
		{"Width", unsafe.Offsetof(attr.Width), 20},
		{"Height", unsafe.Offsetof(attr.Height), 24},
		{"Class", unsafe.Offsetof(attr.Class), 28},
		{"Visual", unsafe.Offsetof(attr.Visual), 32},
		{"WindowType", unsafe.Offsetof(attr.WindowType), 40},
		{"Cursor", unsafe.Offsetof(attr.Cursor), 48},
		{"WMClassName", unsafe.Offsetof(attr.WMClassName), 56},
		{"WMClassClass", unsafe.Offsetof(attr.WMClassClass), 64},
		{"OverrideRedirect", unsafe.Offsetof(attr.OverrideRedirect), 72},
		{"TypeHint", unsafe.Offsetof(attr.TypeHint), 76},
	} {
		if field.got != field.want {
			t.Errorf("GDK window attributes %s offset=%d, want %d", field.name, field.got, field.want)
		}
	}
}
