//go:build darwin || windows || linux

package app

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSetClipboardImagePNGUsesPrivateDriverClipboard(t *testing.T) {
	driver, err := NewDriver(Config{Width: 100, Height: 100}, &gfx.Canvas{})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	driver.SetClipboard("preserve on error")
	if err := SetClipboardImagePNG([]byte("not a PNG")); err == nil {
		t.Fatal("invalid data was accepted")
	}
	if driver.Clipboard() != "preserve on error" {
		t.Fatal("invalid data replaced the clipboard")
	}
	data := onePixelPNG(t)
	if err := SetClipboardImagePNG(data); err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(data)
	data[len(data)-1] ^= 1
	if !bytes.Equal(driver.ClipboardImage(), original) || driver.Clipboard() != "" {
		t.Fatal("clipboard must own a copy of image bytes and replace text")
	}
	if err := SetClipboardText("text"); err != nil {
		t.Fatal(err)
	}
	if len(driver.ClipboardImage()) != 0 {
		t.Fatal("text did not replace image")
	}
}
