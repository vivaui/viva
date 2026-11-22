package linux

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// SetClipboardImage makes png the CLIPBOARD selection, as copying a picture in
// an image viewer does: the X11 owner serves the image/png target while this
// window lives; GTK, which owns the display connection on Wayland, serves the
// same target through its own clipboard. The picture is passed on as the PNG its
// owner wrote, so it keeps the exact bytes and their transparency. Everything
// the clipboard held is gone, as after any copy.
//
// png must be a PNG within platform.MaxClipboardImageBytes, and a clipboard
// selection needs a running window: it lives as long as the process that owns it
// does. The GTK shell serves it without one, because GTK keeps the selection
// itself.
func SetClipboardImage(png []byte) error {
	if err := platform.CheckClipboardPNG(png); err != nil {
		return err
	}
	if hooks := shellHooks.Load(); hooks != nil {
		if hooks.setClipboardImage == nil {
			return ErrUnsupported
		}
		return hooks.setClipboardImage(png)
	}
	win := activeWindow.Load()
	if win == nil {
		return fmt.Errorf("linux: clipboard ownership requires a running window")
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	if activeWindow.Load() != win || win.connection == 0 {
		return fmt.Errorf("linux: clipboard ownership requires a running window")
	}
	if err := win.claimClipboardLocked(); err != nil {
		return err
	}
	win.clipboard = ""
	win.clipboardHTML = ""
	win.clipboardFiles = nil
	win.clipboardImage = append([]byte(nil), png...)
	win.ownsClipboard = true
	return nil
}
