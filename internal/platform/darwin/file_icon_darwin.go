//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "file_icon_darwin.h"
#include "system_darwin.h"
*/
import "C"

import (
	"unsafe"
)

// FileIconBGRA draws the icon Finder shows for the file or folder at path into a
// new pixels x pixels buffer of premultiplied BGRA, top row first. It asks the
// system, so an application bundle shows its own icon and a document the icon of
// its type. A path that does not exist still gets the icon of its extension. It
// may be called from any goroutine.
func FileIconBGRA(path string, pixels int) ([]byte, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	buffer := make([]byte, pixels*pixels*4)
	var nativeError *C.char
	if C.viva_file_icon_bgra(cPath, C.int(pixels), (*C.uchar)(unsafe.Pointer(&buffer[0])), &nativeError) != 0 {
		return nil, systemError("file icon", nativeError)
	}
	return buffer, nil
}
