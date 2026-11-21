package windows

import (
	"time"

	"github.com/vivaui/viva/internal/platform"
)

const (
	// clipboardOpenAttempts is how often a write tries to open the clipboard
	// before giving up: another program holds it only for the moment it takes to
	// put its own data there.
	clipboardOpenAttempts = 5
	// clipboardOpenWait is the pause between those attempts.
	clipboardOpenWait = 10 * time.Millisecond
)

// SetClipboardImage makes png the picture the clipboard holds, as copying an
// image in a browser or Paint does: the registered "PNG" format as it is, and
// CF_DIB converted, so a reader takes the exact copy if it knows PNG and the
// bitmap if it does not. Windows serves the other bitmap formats a reader asks
// for from that CF_DIB block. Everything else the clipboard held is gone, as
// after any copy.
//
// png must be a PNG within platform.MaxClipboardImageBytes and must carry a
// picture a bitmap can hold; platform.CheckClipboardPNG decides the first and
// platform.PNGToDIB the second, and a PNG that fails either leaves the clipboard
// untouched. The bitmap flavour is composited on white, the background a CF_DIB
// reader draws where the picture is transparent; the "PNG" flavour keeps the
// original bytes and their transparency.
func SetClipboardImage(png []byte) error {
	if err := platform.CheckClipboardPNG(png); err != nil {
		return err
	}
	dib, err := platform.PNGToDIB(png)
	if err != nil {
		return err
	}
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	owner := uintptr(0)
	if state := active.Load(); state != nil {
		owner = state.window
	}
	if err := openClipboardForWrite(owner); err != nil {
		return err
	}
	defer closeClipboard.Call()
	if emptied, _, callErr := emptyClipboard.Call(); emptied == 0 {
		return windowsCallError("empty clipboard", callErr)
	}
	if err := setClipboardBlock(clipboardDIB, dib); err != nil {
		return err
	}
	// A clipboard that already holds the bitmap is still a picture without the
	// encoded copy, so a failed registration is not a failed copy; it costs only
	// the transparency the bitmap cannot carry.
	if format := registeredPNGFormat(); format != 0 {
		_ = setClipboardBlock(format, png)
	}
	return nil
}

// openClipboardForWrite opens the clipboard with owner as the window it is held
// for, waiting out the moment another program holds it while it writes.
func openClipboardForWrite(owner uintptr) error {
	var opened uintptr
	var callErr error
	for attempt := 0; attempt < clipboardOpenAttempts; attempt++ {
		if opened, _, callErr = openClipboard.Call(owner); opened != 0 {
			return nil
		}
		time.Sleep(clipboardOpenWait)
	}
	return windowsCallError("open clipboard", callErr)
}
