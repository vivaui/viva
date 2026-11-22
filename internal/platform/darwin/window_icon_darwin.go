//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "status_item_darwin.h"
*/
import "C"

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

func (window *nativeWindow) SetIcon(icon platform.WindowIcon) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("darwin: window is closed")
	}
	tile := icon.Tile(dockIconPixels, platform.MacInset)
	pixels := C.CBytes(tile.RGBA)
	defer C.free(pixels)
	if C.viva_application_update_icon(C.viva_status_icon{size: C.int(tile.Size), rgba: (*C.uchar)(pixels)}) == 0 {
		return fmt.Errorf("darwin: could not update Dock icon")
	}
	return nil
}

// dockIconPixels is the size the Dock icon is made at: 256 points on a 2x screen.
const dockIconPixels = 512

// setApplicationIcon puts the icon in the Dock for a program that is not in an
// .app bundle; a bundle keeps its own.
func setApplicationIcon(icon platform.WindowIcon) {
	if !icon.Valid() {
		return
	}
	// Every Dock icon has a margin around its tile; a full-bleed picture gets one.
	tile := icon.Tile(dockIconPixels, platform.MacInset)
	pixels := C.CBytes(tile.RGBA)
	defer C.free(pixels)
	C.viva_application_set_icon(C.viva_status_icon{size: C.int(tile.Size), rgba: (*C.uchar)(pixels)})
}
