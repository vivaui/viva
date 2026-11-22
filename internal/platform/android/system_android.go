//go:build android

package android

/*
#include <stdlib.h>
#include "jni_android.h"
*/
import "C"

import (
	"strings"
	"unsafe"
)

// What the system answers an application with, from the host's calls into Android's
// services. They are safe from any goroutine.

func hostString(chars *C.char) (string, bool) {
	if chars == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(chars))
	return C.GoString(chars), true
}

// ClipboardText is the text on the clipboard, false when there is none or the system will
// not say (Android lets only the application in front read it).
func ClipboardText() (string, bool) { return hostString(C.viva_host_clipboard_text()) }

// SetClipboardText puts text on the clipboard.
func SetClipboardText(text string) bool {
	units := utf16Units(text)
	var data *C.uint16_t
	if len(units) > 0 {
		data = (*C.uint16_t)(unsafe.Pointer(&units[0]))
	}
	return C.viva_host_set_clipboard_text(data, C.int(len(units))) != 0
}

// OpenURL opens a link in the app that takes it.
func OpenURL(url string) bool {
	units := utf16Units(url)
	if len(units) == 0 {
		return false
	}
	return C.viva_host_open_url((*C.uint16_t)(unsafe.Pointer(&units[0])), C.int(len(units))) != 0
}

// PreferredLanguages are the user's languages, most preferred first, as BCP 47 tags.
func PreferredLanguages() []string {
	tags, ok := hostString(C.viva_host_languages())
	if !ok || tags == "" {
		return nil
	}
	return strings.Split(tags, "\n")
}

// DarkTheme is 1 when the system shows the dark theme, 0 for the light one and -1 when it
// does not say.
func DarkTheme() int { return int(C.viva_host_dark_theme()) }

// AnimationsOff reports whether the user turned animations off.
func AnimationsOff() bool { return C.viva_host_animations_off() != 0 }
