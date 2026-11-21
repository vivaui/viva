//go:build windows

package app

import (
	"bytes"
	"os"
	"testing"
)

// TestSetClipboardImageRoundTripsThroughTheSystemClipboard copies a picture to
// the real clipboard and reads it back with ClipboardImagePNG, the pair an
// application's copy and its paste use. It replaces the clipboard, so it runs
// only on request, on a machine whose clipboard may be overwritten.
func TestSetClipboardImageRoundTripsThroughTheSystemClipboard(t *testing.T) {
	if os.Getenv("VIVA_WINDOWS_CLIPBOARD") == "" {
		t.Skip("set VIVA_WINDOWS_CLIPBOARD=1 to overwrite the real clipboard")
	}
	picture := clipboardPicturePNG(t, 5, 3)
	if err := SetClipboardImage(picture); err != nil {
		t.Fatal(err)
	}
	if !HasClipboardImage() {
		t.Fatal("HasClipboardImage = false after copying a picture")
	}
	read, err := ClipboardImagePNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read, picture) {
		t.Fatalf("read back %d bytes, wrote %d: the clipboard did not keep the copy", len(read), len(picture))
	}

	// A copy replaces what the clipboard held, as the Windows clipboard does
	// after any copy.
	if err := SetClipboardText("text the picture replaces"); err != nil {
		t.Fatal(err)
	}
	if HasClipboardImage() {
		t.Fatal("HasClipboardImage = true with text on the clipboard")
	}
	if err := SetClipboardImage(picture); err != nil {
		t.Fatal(err)
	}
	if text, err := ClipboardText(); err != nil || text != "" {
		t.Fatalf("text = %q, %v after copying a picture, want none", text, err)
	}

	// The asynchronous form writes the same picture.
	if err := SetClipboardText("text the asynchronous copy replaces"); err != nil {
		t.Fatal(err)
	}
	completions, completion := 0, error(nil)
	if err := SetClipboardImageAsync(picture, func(err error) {
		completions++
		completion = err
	}); err != nil {
		t.Fatal(err)
	}
	if completions != 1 || completion != nil {
		t.Fatalf("done ran %d times with %v, want once with nil", completions, completion)
	}
	read, err = ClipboardImagePNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read, picture) {
		t.Fatal("the asynchronous copy did not put the same picture on the clipboard")
	}
}
