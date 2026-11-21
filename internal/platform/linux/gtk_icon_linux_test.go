//go:build linux && vivagtk

package linux

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

func TestGTKIconPixbufKeepsItsPixelsAfterReplacement(t *testing.T) {
	g := &gtkLibrary{}
	for _, name := range []string{"libgdk_pixbuf-2.0.so.0", "libgobject-2.0.so.0", "libglib-2.0.so.0"} {
		library, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			t.Skipf("native pixbuf test needs %s: %v", name, err)
		}
		g.libraries = append(g.libraries, library)
		t.Cleanup(func() { purego.Dlclose(library) })
	}
	g.bind(&g.listAppend, "g_list_append")
	g.bind(&g.listFree, "g_list_free")
	g.bind(&g.objectRef, "g_object_ref")
	g.bind(&g.objectUnref, "g_object_unref")
	var pixels func(uintptr) *byte
	var stride func(uintptr) int32
	var listData func(uintptr, uint32) uintptr
	g.bind(&pixels, "gdk_pixbuf_get_pixels")
	g.bind(&stride, "gdk_pixbuf_get_rowstride")
	g.bind(&listData, "g_list_nth_data")
	if g.err != nil {
		t.Fatal(g.err)
	}
	var retained []uintptr
	g.windowSetIconList = func(_ uintptr, list uintptr) {
		// Hold a reference as another GTK object could, after the window has
		// replaced its icon list.
		pixbuf := listData(list, 0)
		retained = append(retained, g.objectRef(pixbuf))
	}
	t.Cleanup(func() {
		for _, pixbuf := range retained {
			g.objectUnref(pixbuf)
		}
	})
	w := &gtkWindow{g: g, toplevel: 1}
	for _, red := range []byte{50, 200} {
		data := make([]byte, 16*16*4)
		for index := 0; index < len(data); index += 4 {
			copy(data[index:], []byte{red, 100, 30, 255})
		}
		w.config.Icon = platform.WindowIcon{Size: 16, RGBA: data}
		w.setIcon()
	}
	if len(retained) != 2 {
		t.Fatalf("retained %d pixbufs, want two icon updates", len(retained))
	}
	runtime.GC()
	for index, red := range []byte{50, 200} {
		pixbuf := retained[index]
		data := unsafe.Slice(pixels(pixbuf), 15*int(stride(pixbuf))+16*4)
		center := 8*int(stride(pixbuf)) + 8*4
		if data[center] != red || data[center+3] != 255 {
			t.Fatalf("retained icon %d lost its native pixels: %v", index, data[center:center+4])
		}
	}
}
