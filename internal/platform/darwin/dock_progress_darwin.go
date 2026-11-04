//go:build darwin

package darwin

/*
#include "dock_progress_darwin.h"
*/
import "C"

// SetProgress draws how much of the application's work is done along the foot
// of its Dock tile, over the icon the tile already shows, and a negative
// fraction takes the bar away again. The bar belongs to the application, not to
// one window: every window of a program outside an .app bundle shares the tile
// this program draws.
func (application *nativeApplication) SetProgress(fraction float32) {
	C.viva_application_set_progress(C.double(fraction))
}
