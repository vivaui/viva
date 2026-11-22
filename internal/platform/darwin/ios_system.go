//go:build ios

package darwin

/*
#include <stdlib.h>
#include "ios_system.h"
*/
import "C"

import (
	"strings"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// fromC copies and frees a string the native side allocated; false for NULL.
func fromC(value *C.char) (string, bool) {
	if value == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value), true
}

// ClipboardText is the text on the pasteboard, false when there is none.
func ClipboardText() (string, bool) { return fromC(C.viva_ios_clipboard_text()) }

// SetClipboardText puts text on the pasteboard.
func SetClipboardText(text string) {
	native := C.CString(text)
	defer C.free(unsafe.Pointer(native))
	C.viva_ios_set_clipboard_text(native)
}

// OpenURL opens a link in the application that takes it, false when none does.
func OpenURL(url string) bool {
	native := C.CString(url)
	defer C.free(unsafe.Pointer(native))
	return C.viva_ios_open_url(native) != 0
}

// PreferredLanguages are the user's languages, most preferred first, as BCP 47 tags.
func PreferredLanguages() []string {
	joined, ok := fromC(C.viva_ios_preferred_languages())
	if !ok || joined == "" {
		return nil
	}
	return strings.Split(joined, "\n")
}

// ReduceMotion reports whether the user asked for less motion.
func ReduceMotion() bool { return C.viva_ios_reduce_motion() != 0 }

// DarkTheme is 1 when the system shows the dark look, 0 for the light one and -1 when
// it does not say.
func DarkTheme() int { return int(C.viva_ios_dark_theme()) }

//export vivaIOSAppearanceChanged
func vivaIOSAppearanceChanged() { platform.NotifyAppearanceChanged() }
