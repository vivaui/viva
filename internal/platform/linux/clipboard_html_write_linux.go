package linux

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SetClipboardHTML makes the CLIPBOARD selection hold html as its text/html
// target beside alt as the UTF8_STRING one: a rich reader asks for text/html, a
// plain one the text, both from one copy. The X11 owner serves the pair while
// this window lives; GTK, which owns the display connection on Wayland, serves
// the same two targets through its own clipboard.
func SetClipboardHTML(html, alt string) error {
	if len(html) > maxClipboardBytes {
		return fmt.Errorf("linux: clipboard html exceeds 64 MiB")
	}
	if strings.IndexByte(html, 0) >= 0 || strings.IndexByte(alt, 0) >= 0 {
		return fmt.Errorf("linux: clipboard html or text contains NUL")
	}
	if !utf8.ValidString(html) || !utf8.ValidString(alt) {
		return fmt.Errorf("linux: clipboard html is not valid UTF-8")
	}
	if hooks := shellHooks.Load(); hooks != nil {
		if hooks.setClipboardHTML == nil {
			return ErrUnsupported
		}
		return hooks.setClipboardHTML(html, alt)
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
	win.clipboard = alt
	win.clipboardHTML = html
	win.clipboardFiles = nil
	win.ownsClipboard = true
	return nil
}
