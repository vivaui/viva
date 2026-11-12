package linux

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

func testItem() *linuxStatusItem {
	pixels := make([]byte, 4*4*4)
	for index := 0; index < len(pixels); index += 4 {
		copy(pixels[index:], []byte{10, 20, 30, 255})
	}
	return &linuxStatusItem{name: "org.kde.StatusNotifierItem-1-1", tooltip: "Sessions", icon: platform.StatusIcon{Size: 4, RGBA: pixels}}
}

func TestStatusItemPropertiesDecodeAsTheirDeclaredTypes(t *testing.T) {
	item := testItem()
	signatures := map[string]string{
		"Category": "s", "Id": "s", "Title": "s", "Status": "s", "WindowId": "u", "IconName": "s",
		"IconPixmap": "a(iiay)", "OverlayIconName": "s", "OverlayIconPixmap": "a(iiay)",
		"AttentionIconName": "s", "AttentionIconPixmap": "a(iiay)", "AttentionMovieName": "s",
		"ToolTip": "(sa(iiay)ss)", "ItemIsMenu": "b", "Menu": "o",
	}
	for _, name := range propertyNames {
		var e dbus.Encoder
		if !item.writeProperty(&e, name) {
			t.Fatalf("property %s is listed but not written", name)
		}
		d := dbus.NewDecoder(e.Bytes())
		if sig := d.Signature(); sig != signatures[name] {
			t.Fatalf("%s has signature %q, want %q", name, sig, signatures[name])
		}
		d.Skip(signatures[name])
		if d.Err() != nil || !d.Done() {
			t.Fatalf("%s did not decode cleanly to its end: err=%v", name, d.Err())
		}
	}
	if item.writeProperty(&dbus.Encoder{}, "NoSuchProperty") {
		t.Fatal("an unknown property was written")
	}
}

func TestStatusItemPixmapIsARGBAtEachOfferedSize(t *testing.T) {
	var e dbus.Encoder
	testItem().writePixmaps(&e)
	d := dbus.NewDecoder(e.Bytes())
	end := d.ArrayLength(8)
	for _, size := range statusIconSizes {
		d.Align(8)
		if width, height := d.Int32(), d.Int32(); int(width) != size || int(height) != size {
			t.Fatalf("pixmap is %dx%d, want %dx%d", width, height, size, size)
		}
		dataEnd := d.ArrayLength(1)
		if got := dataEnd - d.Pos(); got != size*size*4 {
			t.Fatalf("%d pixel bytes for %dx%d", got, size, size)
		}
		// Alpha first, then red, green, blue: the order the specification names.
		if a, r, g, b := d.Byte(), d.Byte(), d.Byte(), d.Byte(); a != 255 || r != 10 || g != 20 || b != 30 {
			t.Fatalf("first pixel is %d %d %d %d, want ARGB 255 10 20 30", a, r, g, b)
		}
		for d.Pos() < dataEnd {
			d.Byte()
		}
	}
	if d.Pos() != end || d.Err() != nil {
		t.Fatalf("pixmap array ended at %d, want %d (err %v)", d.Pos(), end, d.Err())
	}
}

func TestPanelOpensBesideItsAnchorAndInsideTheScreen(t *testing.T) {
	win := &window{screen: &xcbScreen{WidthPixels: 1920, HeightPixels: 1080}, width: 280, height: 200}
	config := platform.Config{WindowMode: platform.WindowPanel, Width: 280, Height: 200, HasAnchor: true, Anchor: platform.ScreenRect{X: 20, Y: 1055}}
	x, y := win.panelOrigin(config)
	if x < 0 || int(x)+280 > 1920 || y < 0 || int(y)+200 > 1055 {
		t.Fatalf("panel at %d,%d is not above a click at the bottom-left", x, y)
	}
	config.HasAnchor = false
	if x, y := win.panelOrigin(config); x != 820 || y != 440 {
		t.Fatalf("an unanchored panel is at %d,%d, want the screen's center 820,440", x, y)
	}
	config.WindowMode = platform.WindowTitled
	if x, y := win.panelOrigin(config); x != 0 || y != 0 {
		t.Fatalf("an ordinary window was positioned at %d,%d", x, y)
	}
}

// On a scaled display the panel is placed in device pixels (the host reports the
// anchor in them), and a requested origin, in logical pixels, is scaled.
func TestPanelAndOriginAreScaledToDevicePixels(t *testing.T) {
	win := &window{screen: &xcbScreen{WidthPixels: 3840, HeightPixels: 2160}, width: 560, height: 400}
	win.storeScale(2)
	config := platform.Config{WindowMode: platform.WindowPanel, Width: 280, Height: 200}
	if x, y := win.panelOrigin(config); x != 1640 || y != 880 {
		t.Fatalf("an unanchored 280x200 panel at 2x is at %d,%d, want the center of the screen 1640,880", x, y)
	}
	config = platform.Config{WindowMode: platform.WindowTitled, Width: 280, Height: 200, HasOrigin: true, Origin: gfx.Point{X: 100, Y: 50.5}}
	if x, y := win.panelOrigin(config); x != 200 || y != 101 {
		t.Fatalf("origin 100,50.5 at 2x is %d,%d, want 200,101", x, y)
	}
}
