//go:build darwin && vivahead && !ios

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// OverrideDefaultInteger sets the integer user default key for this process only (remove
// clears it): what the user's `defaults write` would change, without touching their
// preferences. It exists for tests of settings AppKit reads, like AppleFontSmoothing.
func OverrideDefaultInteger(key string, value int, remove bool) {
	name := C.CString(key)
	defer C.free(unsafe.Pointer(name))
	var flag C.int
	if remove {
		flag = 1
	}
	C.viva_override_default_integer(name, C.int(value), flag)
}

// ReferenceText draws value in white, in one native call for the whole string, as a macOS
// application draws a line of text, into a width x height device-pixel bitmap at scale
// (premultiplied BGRA, top row first), on the baseline a label has. It is what the label
// drawing is measured against for sharpness; like Offscreen it exists only in builds with
// -tags vivahead.
func ReferenceText(value string, size, weight, scale float64, width, height int) ([]byte, error) {
	if value == "" || size <= 0 || scale < 1 || width < 1 || height < 1 {
		return nil, fmt.Errorf("darwin: invalid reference text request")
	}
	pixels := make([]byte, width*height*4)
	text := []byte(value)
	if C.viva_reference_text((*C.uchar)(unsafe.Pointer(&text[0])), C.size_t(len(text)), C.double(size), C.double(weight), C.double(scale),
		C.size_t(width), C.size_t(height), (*C.uchar)(unsafe.Pointer(&pixels[0]))) != 0 {
		return nil, fmt.Errorf("darwin: reference text drawing failed")
	}
	return pixels, nil
}
