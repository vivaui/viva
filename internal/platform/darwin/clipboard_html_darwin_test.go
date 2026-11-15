//go:build darwin

package darwin

import (
	"testing"
)

// TestPasteboardHTMLOffersBothFlavours writes the pair on a private pasteboard
// and reads each flavour back the way an application that asks for it would:
// an html reader takes public.html, a plain reader the text beside it.
func TestPasteboardHTMLOffersBothFlavours(t *testing.T) {
	name := privateName(t)
	const html = `<b>加粗</b> &amp; <span>text</span>`
	const alt = "加粗 & text"
	if got, err := privatePasteboardHTML(name); err != nil || got != "" {
		t.Fatalf("a fresh pasteboard: html = %q, %v", got, err)
	}
	if err := putPrivatePasteboardHTML(name, html, alt); err != nil {
		t.Fatal(err)
	}
	got, err := privatePasteboardHTML(name)
	if err != nil || got != html {
		t.Fatalf("pasteboard html = %q, %v, want %q", got, err, html)
	}
	text, err := privatePasteboardText(name)
	if err != nil || text != alt {
		t.Fatalf("pasteboard text beside the html = %q, %v, want %q", text, err, alt)
	}

	// A copy of another kind replaces the pair, as every write clears the board.
	if err := putPrivatePasteboardFiles(name, []string{"/tmp/report.txt"}); err != nil {
		t.Fatal(err)
	}
	if got, err := privatePasteboardHTML(name); err != nil || got != "" {
		t.Fatalf("after files replaced the pair: html = %q, %v", got, err)
	}
}
