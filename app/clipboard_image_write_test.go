//go:build darwin || windows || linux

package app

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// clipboardPicturePNG encodes a picture whose colours and edges a round trip has
// to keep apart: a grid that differs in every channel along both axes.
func clipboardPicturePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.SetNRGBA(x, y, color.NRGBA{R: uint8(30 + 50*x), G: uint8(10 + 60*y), B: 200, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestSetClipboardImageCopiesOntoTheDriverClipboard checks the write against the
// driver's private clipboard, which answers the same reads as the system one:
// the picture is there, the reads see it, and the text a copy replaces is gone.
func TestSetClipboardImageCopiesOntoTheDriverClipboard(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	driver.SetClipboard("text the copy replaces")

	picture := clipboardPicturePNG(t, 3, 2)
	if err := SetClipboardImage(picture); err != nil {
		t.Fatal(err)
	}
	if got := driver.ClipboardImage(); !bytes.Equal(got, picture) {
		t.Fatalf("driver clipboard holds %d bytes, want the %d written", len(got), len(picture))
	}
	if driver.Clipboard() != "" {
		t.Fatalf("the text stayed on the clipboard: %q", driver.Clipboard())
	}
	if !HasClipboardImage() {
		t.Fatal("HasClipboardImage = false after a copy")
	}
	read, err := ClipboardImagePNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read, picture) {
		t.Fatalf("read back %d bytes, want the %d written", len(read), len(picture))
	}
}

// TestSetClipboardImageAsyncCopiesOnce checks the asynchronous form of the same
// write on a native target: done runs before the call returns, exactly once, and
// the clipboard holds the picture.
func TestSetClipboardImageAsyncCopiesOnce(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	picture := clipboardPicturePNG(t, 2, 2)
	completions, completion := 0, errors.New("done never ran")
	if err := SetClipboardImageAsync(picture, func(err error) {
		completions++
		completion = err
	}); err != nil {
		t.Fatal(err)
	}
	if completions != 1 || completion != nil {
		t.Fatalf("done ran %d times with %v, want once with nil", completions, completion)
	}
	if !bytes.Equal(driver.ClipboardImage(), picture) {
		t.Fatal("the asynchronous copy left the clipboard without the picture")
	}
}

// TestSetClipboardImageRefusesWhatItCannotCopy keeps a failed write from lying
// about the clipboard: an empty image, a file that is not a PNG and one above
// the cap are errors, and the copy already there stays.
func TestSetClipboardImageRefusesWhatItCannotCopy(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	picture := clipboardPicturePNG(t, 2, 1)
	if err := SetClipboardImage(picture); err != nil {
		t.Fatal(err)
	}
	if err := SetClipboardImage(nil); err == nil {
		t.Fatal("an empty image was copied")
	}
	if err := SetClipboardImage([]byte("GIF89a, not a PNG")); err == nil {
		t.Fatal("bytes that are not a PNG were copied")
	}
	aboveCap := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxClipboardImageBytes)...)
	if err := SetClipboardImage(aboveCap); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
	if got := driver.ClipboardImage(); !bytes.Equal(got, picture) {
		t.Fatal("a refused copy replaced the picture on the clipboard")
	}
	if err := SetClipboardImageAsync(nil, func(error) {}); err == nil {
		t.Fatal("an empty image was accepted asynchronously")
	}
	if err := SetClipboardImageAsync(picture, nil); err == nil {
		t.Fatal("a nil callback was accepted")
	}
}
