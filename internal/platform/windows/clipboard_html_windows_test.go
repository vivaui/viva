package windows

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestCFHTMLHeaderPointsAtTheDocument pins the CF_HTML block to the shape
// readers slice the fragment out of: the header's byte offsets are counted from
// the start of the block, and the fragment they name is the html the caller
// passed, wrapped the way the format's description wraps it.
func TestCFHTMLHeaderPointsAtTheDocument(t *testing.T) {
	const html = `<b>加粗</b> and text`
	block := cfHTML(html)
	whole := string(block)
	if !strings.HasPrefix(whole, "Version:0.9\r\nStartHTML:") {
		t.Fatalf("the block does not begin with the header: %q", whole[:40])
	}
	field := func(name string) int {
		t.Helper()
		mark := strings.Index(whole, name+":")
		if mark < 0 {
			t.Fatalf("the header has no %s", name)
		}
		end := mark + len(name) + 1
		value, err := strconv.Atoi(whole[end : end+10])
		if err != nil {
			t.Fatalf("%s is not ten digits: %q", name, whole[end:end+10])
		}
		return value
	}
	startHTML := field("StartHTML")
	endHTML := field("EndHTML")
	startFragment := field("StartFragment")
	endFragment := field("EndFragment")
	// The header's own length: each line is its field name, ten digits and a
	// CRLF, so the document starts right behind them.
	const headerLength = len("Version:0.9\r\n") +
		len("StartHTML:") + 10 + 2 +
		len("EndHTML:") + 10 + 2 +
		len("StartFragment:") + 10 + 2 +
		len("EndFragment:") + 10 + 2
	if startHTML != headerLength {
		t.Fatalf("StartHTML = %d, want the header's own length %d: %q", startHTML, headerLength, whole[:startHTML])
	}
	if !strings.HasPrefix(whole[startHTML:], "<html><body>") {
		t.Fatalf("the document does not start at StartHTML: %q", whole[startHTML:])
	}
	if string(block[startFragment:endFragment]) != html {
		t.Fatalf("the fragment = %q, want %q", block[startFragment:endFragment], html)
	}
	if !strings.HasSuffix(string(block[startHTML:endHTML]), "</body>\r\n</html>") {
		t.Fatalf("the document does not end at EndHTML: %q", whole[startHTML:endHTML])
	}
}

// TestSetClipboardHTMLRoundTrip writes both flavours and has the clipboard read
// them the way an application pastes: CF_UNICODETEXT as the plain text,
// "HTML Format" as the CF_HTML. It replaces the clipboard, so it runs only on
// request, on a machine whose clipboard may be overwritten.
func TestSetClipboardHTMLRoundTrip(t *testing.T) {
	if os.Getenv("VIVA_WINDOWS_CLIPBOARD") == "" {
		t.Skip("set VIVA_WINDOWS_CLIPBOARD=1 to overwrite the real clipboard")
	}
	const html = `<b>加粗</b> and text`
	const alt = "加粗 and text"
	if err := SetClipboardHTML(html, alt); err != nil {
		t.Fatal(err)
	}
	text, err := ClipboardText()
	if err != nil || text != alt {
		t.Fatalf("plain flavour = %q, %v, want %q", text, err, alt)
	}
	if available, _, _ := clipboardFormatAvailable.Call(registeredHTMLFormat()); available == 0 {
		t.Fatal("the clipboard holds no HTML Format flavour")
	}
	block, err := clipboardBytes(registeredHTMLFormat(), maxClipboardBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(block), html) {
		t.Fatalf("the CF_HTML block does not carry the html: %q", block)
	}

	// Another copy replaces the pair, as every write clears the clipboard.
	if err := SetClipboardText("plain again"); err != nil {
		t.Fatal(err)
	}
	if available, _, _ := clipboardFormatAvailable.Call(registeredHTMLFormat()); available != 0 {
		t.Fatal("a plain copy left a rich flavour behind")
	}
}
