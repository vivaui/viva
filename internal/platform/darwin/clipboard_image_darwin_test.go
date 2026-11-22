//go:build darwin && !ios

package darwin

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	picture.SetNRGBA(0, 0, color.NRGBA{200, 10, 20, 255})
	picture.SetNRGBA(2, 1, color.NRGBA{0, 90, 180, 255})
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func privateName(t *testing.T) string {
	return fmt.Sprintf("viva.test.%s.%d", t.Name(), time.Now().UnixNano())
}

func TestPasteboardImageFormats(t *testing.T) {
	source := testPNG(t)
	for _, tiff := range []bool{false, true} {
		name := privateName(t)
		if privatePasteboardHasImage(name) {
			t.Fatal("a fresh pasteboard reports an image")
		}
		if _, err := privatePasteboardImage(name); !errors.Is(err, platform.ErrNoClipboardImage) {
			t.Fatalf("empty pasteboard error = %v, want ErrNoClipboardImage", err)
		}
		if err := putPrivatePasteboardImage(name, source, tiff); err != nil {
			t.Fatal(err)
		}
		if !privatePasteboardHasImage(name) {
			t.Fatalf("tiff=%v: pasteboard with an image reports none", tiff)
		}
		got, err := privatePasteboardImage(name)
		if err != nil {
			t.Fatalf("tiff=%v: %v", tiff, err)
		}
		decoded, err := png.Decode(bytes.NewReader(got))
		if err != nil {
			t.Fatalf("tiff=%v: result is not a PNG: %v", tiff, err)
		}
		if decoded.Bounds().Dx() != 3 || decoded.Bounds().Dy() != 2 {
			t.Fatalf("tiff=%v: size = %v", tiff, decoded.Bounds())
		}
		r, g, b, a := decoded.At(0, 0).RGBA()
		if r>>8 < 190 || r>>8 > 210 || g>>8 > 20 || b>>8 > 30 || a>>8 != 255 {
			t.Fatalf("tiff=%v: pixel (0,0) = %d %d %d %d", tiff, r>>8, g>>8, b>>8, a>>8)
		}
	}
}
