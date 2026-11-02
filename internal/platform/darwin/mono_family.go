//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/ui"
)

var (
	monoFamilyLock sync.Mutex
	monoFamilySent int
)

// syncMonoFamilies tells the native side what the faces the application registered
// with ui.MonoFamily and ui.MonoStack since the last call stand for. Faces are numbered
// in registration order, so only the new ones are sent.
func syncMonoFamilies() {
	count := ui.MonoFaceCount()
	monoFamilyLock.Lock()
	defer monoFamilyLock.Unlock()
	for monoFamilySent < count {
		face := ui.MonoFace(2 + monoFamilySent)
		name := C.CString(ui.MonoFamilyName(face))
		fallbacks := C.CString(strings.Join(ui.MonoFallbacks(face), "\n"))
		classic := C.int(0)
		if ui.MonoIsClassic(face) {
			classic = 1
		}
		C.viva_register_mono_family(C.int(2+monoFamilySent), name, classic, fallbacks)
		C.free(unsafe.Pointer(name))
		C.free(unsafe.Pointer(fallbacks))
		monoFamilySent++
	}
}

// FontFamilies lists every installed font family, sorted.
func FontFamilies() []string {
	list := C.viva_list_font_families()
	if list == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(list))
	value := C.GoString(list)
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

// MonospaceFamilies lists the installed font families that draw on a character grid,
// sorted. A family counts when it says it is fixed-pitch or when its printable ASCII
// advances are measured to share one width; other platforms trust what the system
// declares.
func MonospaceFamilies() []string {
	list := C.viva_list_mono_families()
	if list == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(list))
	value := C.GoString(list)
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
