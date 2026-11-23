//go:build darwin && vivahead && !ios

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import "unsafe"

// ProbeType delivers one key press carrying text to the running window's view
// from inside the process, as AppKit would after the window server handed it
// the event. It never creates a system-level event. Like Offscreen it exists
// only in builds with -tags vivahead, so release builds link none of it.
func ProbeType(text string) {
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	C.viva_probe_type(value)
}

// ProbeFront raises the running window above all others for the length of a
// measurement; a window another application covers is not refreshed, which
// would spoil the numbers.
func ProbeFront() { C.viva_probe_front() }

// ProbeClose closes the running window, ending Run.
func ProbeClose() { C.viva_probe_close() }
