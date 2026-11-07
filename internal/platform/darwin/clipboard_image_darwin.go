//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "system_darwin.h"
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"strings"
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
	return putPasteboardFiles(cName, paths)
}

// SetClipboardFiles puts paths on the general pasteboard, as Finder does after
// the user copies a selection.
func SetClipboardFiles(paths []string) error {
	return putPasteboardFiles(nil, paths)
}

func putPasteboardFiles(name *C.char, paths []string) error {
	names := make([]string, len(paths))
	for index, path := range paths {
		names[index] = filepath.Base(path)
	}
	cNames := C.CString(strings.Join(names, "\n"))
	defer C.free(unsafe.Pointer(cNames))
	native := make([]*C.char, len(paths))
	for index, path := range paths {
		native[index] = C.CString(path)
	}
	defer func() {
		for _, value := range native {
			C.free(unsafe.Pointer(value))
		}
	}()
	if C.viva_system_pasteboard_put_files(name, (**C.char)(unsafe.Pointer(&native[0])), C.size_t(len(native)), cNames) != 0 {
		if name == nil {
			return fmt.Errorf("darwin: put files on the clipboard failed")
		}
		return fmt.Errorf("darwin: put files on pasteboard %q failed", C.GoString(name))
	}
	return nil
}
