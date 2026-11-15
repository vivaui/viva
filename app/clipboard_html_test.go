//go:build darwin || windows || linux

package app

import (
	"testing"
)

// TestDriverSetClipboardHoldsBothFlavours exercises the headless store of a
// rich copy: both flavours are answered by their own readers, the plain one is
// the driver's clipboard (the text path a paste already uses), and anything
// else copied replaces the pair, as on a real clipboard.
func TestDriverSetClipboardHoldsBothFlavours(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	const html = `<b>bold</b> and 中文`
	const alt = "bold and 中文"
	if err := SetClipboardHTML(html, alt); err != nil {
		t.Fatal(err)
	}
	if got := driver.ClipboardHTML(); got != html {
		t.Fatalf("driver html = %q, want %q", got, html)
	}
	if text, err := ClipboardText(); err != nil || text != alt {
		t.Fatalf("plain flavour = %q, %v, want %q", text, err, alt)
	}
	if err := SetClipboardText("typed"); err != nil {
		t.Fatal(err)
	}
	if got := driver.ClipboardHTML(); got != "" {
		t.Fatalf("after text replaced the pair: html = %q, want empty", got)
	}

	driver.SetClipboardHTML("<b>one</b>", "one")
	if err := driver.SetClipboardImage(onePixelPNG(t)); err != nil {
		t.Fatal(err)
	}
	if got := driver.ClipboardHTML(); got != "" {
		t.Fatalf("after an image replaced the pair: html = %q, want empty", got)
	}

	// The pre-seeded half of the pair: tests put a rich copy on the private
	// clipboard the way another application would.
	driver.SetClipboardHTML("<code>y</code>", "y")
	if got := driver.ClipboardHTML(); got != "<code>y</code>" {
		t.Fatalf("SetClipboardHTML of the driver = %q", got)
	}
	if text, err := ClipboardText(); err != nil || text != "y" {
		t.Fatalf("after the seeded pair: text = %q, %v", text, err)
	}
}

// TestSetClipboardHTMLDegradesToSetClipboardText pins the empty-html path to
// SetClipboardText's own shape: the copy is plain either way, and a refused
// copy writes nothing.
func TestSetClipboardHTMLDegradesToSetClipboardText(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := SetClipboardHTML("", "plain only"); err != nil {
		t.Fatal(err)
	}
	if text, err := ClipboardText(); err != nil || text != "plain only" {
		t.Fatalf("plain flavour = %q, %v", text, err)
	}
	if got := driver.ClipboardHTML(); got != "" {
		t.Fatalf("an empty html left a rich flavour behind: %q", got)
	}

	if err := SetClipboardHTML("", ""); err != nil {
		t.Fatal(err)
	}
	if text, err := ClipboardText(); err != nil || text != "" {
		t.Fatalf("empty copy: text = %q, %v", text, err)
	}

	if err := SetClipboardHTML("a\x00b", "alt"); err == nil {
		t.Fatal("html with NUL was accepted")
	}
	if err := SetClipboardHTML("<b>x</b>", "a\x00b"); err == nil {
		t.Fatal("alt with NUL was accepted")
	}
	if text, _ := ClipboardText(); text != "" {
		t.Fatalf("a refused copy wrote %q", text)
	}
}

// TestSetClipboardHTMLAsyncOfADriverRun pins the async write to the same store;
// on native targets it completes before the call returns.
func TestSetClipboardHTMLAsyncOfADriverRun(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	var written bool
	if err := SetClipboardHTMLAsync("<b>q</b>", "q", func(err error) {
		written = true
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("the async write did not complete on a driver-backed run")
	}
	if got := driver.ClipboardHTML(); got != "<b>q</b>" {
		t.Fatalf("driver html = %q", got)
	}
	if text, err := ClipboardText(); err != nil || text != "q" {
		t.Fatalf("plain flavour = %q, %v", text, err)
	}

	if err := SetClipboardHTMLAsync("", "plain", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if got := driver.ClipboardHTML(); got != "" {
		t.Fatalf("an empty async html left a rich flavour behind: %q", got)
	}
	if text, _ := ClipboardText(); text != "plain" {
		t.Fatalf("after the empty-html async copy: text = %q", text)
	}
}
