package windows

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

// SetClipboardHTML writes html as the clipboard's registered "HTML Format"
// (CF_HTML) beside alt as CF_UNICODETEXT: a rich reader takes the CF_HTML, a
// plain one the text, both from one copy. The CF_HTML header carries byte
// offsets into its own block, which the format pins to ten digits, so its
// length is the same whatever they say and one pass fixes every offset.
func SetClipboardHTML(html, alt string) error {
	if len(html) > maxClipboardBytes {
		return fmt.Errorf("windows: clipboard html exceeds 64 MiB")
	}
	encoded, err := syscall.UTF16FromString(alt)
	if err != nil {
		return fmt.Errorf("windows: encode clipboard text: %w", err)
	}
	// The block API takes bytes; the UTF-16 the clipboard wants is the same
	// units little-endian, the encoding the file list's names already use.
	text := make([]byte, len(encoded)*2)
	for index, unit := range encoded {
		binary.LittleEndian.PutUint16(text[index*2:], unit)
	}
	block := cfHTML(html)
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	owner := uintptr(0)
	if state := active.Load(); state != nil {
		owner = state.window
	}
	if opened, _, callErr := openClipboard.Call(owner); opened == 0 {
		return windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	if emptied, _, callErr := emptyClipboard.Call(); emptied == 0 {
		return windowsCallError("empty clipboard", callErr)
	}
	if err := setClipboardBlock(clipboardUnicodeText, text); err != nil {
		return err
	}
	return registeredSetClipboardHTML(block)
}

// registeredSetClipboardHTML puts the CF_HTML block of a rich copy on the open
// clipboard, the registered "HTML Format" every CF_HTML reader looks for.
func registeredSetClipboardHTML(block []byte) error {
	format := registeredHTMLFormat()
	if format == 0 {
		return fmt.Errorf("windows: register clipboard format failed")
	}
	return setClipboardBlock(format, block)
}

var htmlFormatOnce sync.Once
var htmlFormat uintptr

// registeredHTMLFormat is the clipboard format named "HTML Format"; 0 when it
// cannot be registered.
func registeredHTMLFormat() uintptr {
	htmlFormatOnce.Do(func() {
		name, _ := syscall.UTF16PtrFromString("HTML Format")
		htmlFormat, _, _ = registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	})
	return htmlFormat
}

// cfHTML builds the CF_HTML block around html: a header of byte offsets and
// the document they point into, exactly the shape the format's published
// description builds (the fragment wrapped in body comments, offsets counted
// from the start of the block, in UTF-8).
func cfHTML(html string) []byte {
	const (
		front = "<html><body>\r\n<!--StartFragment-->"
		back  = "<!--EndFragment-->\r\n</body>\r\n</html>"
		// The header is fixed once its values are ten digits wide; the
		// numbers only name byte offsets, never change the width.
		lands = "Version:0.9\r\n" +
			"StartHTML:0000000000\r\n" +
			"EndHTML:0000000000\r\n" +
			"StartFragment:0000000000\r\n" +
			"EndFragment:0000000000\r\n"
	)
	startHTML := len(lands)
	startFragment := startHTML + len(front)
	endFragment := startFragment + len(html)
	endHTML := endFragment + len(back)
	block := make([]byte, 0, endHTML)
	block = append(block, "Version:0.9\r\n"...)
	block = strconv.AppendUint(block, uint64(startHTML), 10)
	block = append(block, "\r\nEndHTML:"...)
	block = strconv.AppendUint(block, uint64(endHTML), 10)
	block = append(block, "\r\nStartFragment:"...)
	block = strconv.AppendUint(block, uint64(startFragment), 10)
	block = append(block, "\r\nEndFragment:"...)
	block = strconv.AppendUint(block, uint64(endFragment), 10)
	block = append(block, "\r\n"...)
	block = append(block, front...)
	block = append(block, html...)
	block = append(block, back...)
	return block
}
