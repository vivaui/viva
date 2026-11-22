//go:build darwin && !ios

package darwin

/*
#include <stdlib.h>
#include "dock_badge_darwin.h"
*/
import "C"

import "unsafe"

// SetDockBadge puts text in the red badge macOS draws at the top corner of the
// Dock tile, and an empty text takes the badge away. The badge belongs to the
// application, not to one window: every window of a program outside an .app
// bundle shares the Dock tile this program sets.
func (application *nativeApplication) SetDockBadge(text string) {
	if text == "" {
		C.viva_application_set_badge_label(nil)
		return
	}
	label := C.CString(text)
	defer C.free(unsafe.Pointer(label))
	C.viva_application_set_badge_label(label)
}
