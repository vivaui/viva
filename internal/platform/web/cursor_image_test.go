package web

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCursorImageCSSNamesTheHotspotAndFallsBack(t *testing.T) {
	image, err := ui.NewCursorImage(2, 2, 1, 0, 1, make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	css, err := cursorImageCSS(image)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(css, `url("data:image/png;base64,`) || !strings.HasSuffix(css, `") 1 0, default`) {
		t.Fatalf("css = %q", css)
	}
}

func TestCursorImageCSSOffersAScaledPictureAtItsResolution(t *testing.T) {
	image, err := ui.NewCursorImage(4, 4, 2, 2, 2, make([]byte, 64))
	if err != nil {
		t.Fatal(err)
	}
	css, err := cursorImageCSS(image)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(css, "image-set(url(") || !strings.HasSuffix(css, `") 2x) 1 1, default`) {
		t.Fatalf("css = %q", css)
	}
}
