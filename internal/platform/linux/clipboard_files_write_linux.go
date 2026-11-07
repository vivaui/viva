package linux

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// SetClipboardFiles makes paths the CLIPBOARD selection, as a file manager does
// after the user copies a selection. The X11 owner serves text/uri-list and the
// x-special/gnome-copied-files form while this window lives; GTK, which owns
// the display connection on Wayland, serves the same list through its own
// clipboard.
func SetClipboardFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("linux: no files to put on the clipboard")
	}
	for index, path := range paths {
		if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 || !utf8.ValidString(path) {
			return fmt.Errorf("linux: clipboard file %d is not an absolute UTF-8 path: %q", index, path)
		}
	}
	if hooks := shellHooks.Load(); hooks != nil {
		if hooks.setClipboardFiles == nil {
			return ErrUnsupported
		}
		return hooks.setClipboardFiles(paths)
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
	names := make([]string, len(paths))
	for index, path := range paths {
		names[index] = filepath.Base(path)
	}
	win.clipboard = strings.Join(names, "\n")
	win.clipboardFiles = append([]string(nil), paths...)
	win.ownsClipboard = true
	return nil
}

// claimClipboardLocked makes an existing window the CLIPBOARD owner, after the
// atoms it serves have been interned. win.mu is held.
func (win *window) claimClipboardLocked() error {
	if err := win.ensureClipboardAtoms(); err != nil {
		return err
	}
	win.api.setSelectionOwner(win.connection, win.id, win.clipboardAtom, 0)
	if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
		return fmt.Errorf("linux: claim clipboard: XCB connection failed")
	}
	owner, err := selectionOwner(win.api, win.connection, win.clipboardAtom)
	if err != nil {
		return err
	}
	if owner != win.id {
		return fmt.Errorf("linux: X server refused clipboard ownership")
	}
	return nil
}
