//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework CoreText -framework CoreFoundation
#include <CoreText/CoreText.h>
#include <stdlib.h>

// viva_font_scope_file registers (add) or unregisters the font file at path for this process only,
// and reports whether the font manager accepted it.
static int viva_font_scope_file(const char *path, int add) {
	CFStringRef text = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
	if (text == NULL) {
		return 0;
	}
	CFURLRef url = CFURLCreateWithFileSystemPath(NULL, text, kCFURLPOSIXPathStyle, false);
	CFRelease(text);
	if (url == NULL) {
		return 0;
	}
	CFErrorRef error = NULL;
	bool ok = add ? CTFontManagerRegisterFontsForURL(url, kCTFontManagerScopeProcess, &error)
	              : CTFontManagerUnregisterFontsForURL(url, kCTFontManagerScopeProcess, &error);
	if (error != NULL) {
		CFRelease(error);
	}
	CFRelease(url);
	return ok ? 1 : 0;
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/fontfile"
)

var fonts fontfile.Registry

var _ gfx.FontRegistrar = (*TextEngine)(nil)

// RegisterFont implements gfx.FontRegistrar: the font is registered with CoreText for this
// process, so the text stack finds its family like an installed one, the fallback included.
func (engine *TextEngine) RegisterFont(data []byte) (string, func(), error) {
	family, unregister, err := fonts.Register(data, func(path string) error {
		name := C.CString(path)
		defer C.free(unsafe.Pointer(name))
		if C.viva_font_scope_file(name, 1) == 0 {
			return errors.New("the font manager refused the font")
		}
		return nil
	}, func(path string) {
		name := C.CString(path)
		defer C.free(unsafe.Pointer(name))
		C.viva_font_scope_file(name, 0)
	})
	if err != nil {
		return "", nil, err
	}
	engine.Reset()
	return family, func() {
		unregister()
		engine.Reset()
	}, nil
}
