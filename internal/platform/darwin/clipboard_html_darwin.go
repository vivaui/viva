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

// SetClipboardHTML puts html on the general pasteboard as public.html beside
// alt as public.utf8-plain-text, the pair a rich copy leaves. The flavours are
// the pasteboard's own types, so a reader takes whichever it knows.
func SetClipboardHTML(html, alt string) error { return putPasteboardHTML(nil, html, alt) }

func putPasteboardHTML(name *C.char, html, alt string) error {
	nativeHTML, err := systemCString("clipboard html", html)
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(nativeHTML))
	nativeAlt, err := systemCString("clipboard alt", alt)
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(nativeAlt))
	if C.viva_system_pasteboard_put_html(name, nativeHTML, nativeAlt) != 0 {
		if name == nil {
			return fmt.Errorf("darwin: put html on the clipboard failed")
		}
		return fmt.Errorf("darwin: put html on pasteboard %q failed", C.GoString(name))
	}
	return nil
}

func putPrivatePasteboardHTML(name, html, alt string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return putPasteboardHTML(cName, html, alt)
}

func privatePasteboardHTML(name string) (string, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	var value *C.char
	var nativeError *C.char
	if C.viva_system_pasteboard_html(cName, &value, &nativeError) != 0 {
		return "", systemError("read pasteboard html", nativeError)
	}
	defer C.viva_system_free(unsafe.Pointer(value))
	return C.GoString(value), nil
}

func privatePasteboardText(name string) (string, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	var value *C.char
	var nativeError *C.char
	if C.viva_system_pasteboard_text(cName, &value, &nativeError) != 0 {
		return "", systemError("read pasteboard text", nativeError)
	}
	defer C.viva_system_free(unsafe.Pointer(value))
	return C.GoString(value), nil
}
