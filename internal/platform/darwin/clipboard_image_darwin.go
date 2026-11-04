//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "system_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// The conversion is exercised on private pasteboards, never the user's
// clipboard: NSPasteboard pasteboardWithName: makes one the tests own.

func privatePasteboardImage(name string) ([]byte, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return pasteboardImagePNG(cName)
}

func privatePasteboardHasImage(name string) bool {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return C.viva_system_clipboard_has_image(cName) != 0
}

func putPrivatePasteboardImage(name string, png []byte, asTIFF bool) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	tiff := C.int(0)
	if asTIFF {
		tiff = 1
	}
	if C.viva_system_pasteboard_put_image(cName, (*C.uint8_t)(unsafe.Pointer(&png[0])), C.size_t(len(png)), tiff) != 0 {
		return fmt.Errorf("darwin: put image on pasteboard %q failed", name)
	}
	return nil
}

func privatePasteboardFiles(name string) ([]string, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return pasteboardFiles(cName)
}

func putPrivatePasteboardFiles(name string, paths []string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	native := make([]*C.char, len(paths))
	for index, path := range paths {
		native[index] = C.CString(path)
	}
	defer func() {
		for _, value := range native {
			C.free(unsafe.Pointer(value))
		}
	}()
	if C.viva_system_pasteboard_put_files(cName, (**C.char)(unsafe.Pointer(&native[0])), C.size_t(len(native))) != 0 {
		return fmt.Errorf("darwin: put files on pasteboard %q failed", name)
	}
	return nil
}
