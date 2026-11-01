//go:build darwin && vivahead

package darwin

/*
#include <stdlib.h>

int viva_register_font_file(const char *path);
int viva_unregister_font_file(const char *path);
*/
import "C"

import (
	"errors"
	"unsafe"
)

// RegisterFontFile makes the font in path available to this process's font
// manager, and returns a function that takes it away again. Process registration
// is how a headless run or a test draws a font it carries along without
// installing it in the user's font library; the text stack then finds the family
// like any other, the fallback for a character no installed family draws
// included.
//
// It exists only in builds with -tags vivahead, beside the offscreen renderer it
// is meant for. Registering a file twice, or unregistering one that is not
// registered, is an error.
func RegisterFontFile(path string) (unregister func(), err error) {
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	if C.viva_register_font_file(name) != 0 {
		return nil, errors.New("register font file " + path)
	}
	return func() {
		name := C.CString(path)
		defer C.free(unsafe.Pointer(name))
		C.viva_unregister_font_file(name)
	}, nil
}
