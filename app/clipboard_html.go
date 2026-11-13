package app

import (
	"fmt"
	"strings"
)

// SetClipboardHTML makes the system clipboard hold html as its rich flavour and
// alt as its plain one, in a single write, for a copy a reader may take either
// way: a query result that pastes with its colours into a document and as text
// into an editor. alt is what plain-text readers get — deriving it from html is
// the caller's job, not the clipboard's.
//
// The flavours are what each platform calls rich text: NSPasteboard's
// public.html beside public.utf8-plain-text, Windows' registered "HTML Format"
// (CF_HTML) with the plain text as CF_UNICODETEXT, and the CLIPBOARD
// selection's text/html target beside UTF8_STRING. Nothing is written when
// either string contains NUL; an empty html writes alt as plain text only, with
// exactly SetClipboardText's shape, so the degradation is the copy the caller
// already spelled out. The Web target cannot answer a synchronous write
// (Clipboard.write resolves a Promise) and returns ErrAsyncRequired;
// SetClipboardHTMLAsync writes both flavours there.
func SetClipboardHTML(html, alt string) error {
	if strings.IndexByte(html, 0) >= 0 {
		return fmt.Errorf("app: clipboard html contains NUL")
	}
	if strings.IndexByte(alt, 0) >= 0 {
		return fmt.Errorf("app: clipboard text contains NUL")
	}
	if html == "" {
		return SetClipboardText(alt)
	}
	if err, handled := driverSetClipboardHTML(html, alt); handled {
		return err
	}
	return nativeSetClipboardHTML(html, alt)
}
