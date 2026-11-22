package platform

import "testing"

func TestNotificationPlacementRespectsWorkAreaAndNegativeScreenOrigins(t *testing.T) {
	area := ScreenRect{X: -1920, Y: 36, Width: 1920, Height: 1000}
	if x, y := PlaceNotification(320, 128, 12, area); x != -332 || y != 896 {
		t.Fatalf("notification at %g,%g", x, y)
	}
	if x, y := PlaceNotification(2000, 1200, 12, area); x != area.X || y != area.Y {
		t.Fatalf("oversize notification at %g,%g", x, y)
	}
}

func TestPlacePanelOpensTowardTheMiddleOfTheScreen(t *testing.T) {
	work := ScreenRect{Width: 1920, Height: 1040} // a 40 pixel taskbar at the bottom
	// An icon in the bottom-right corner of a bottom taskbar: the panel is above
	// it and against the right edge.
	icon := ScreenRect{X: 1850, Y: 1048, Width: 32, Height: 32}
	x, y := PlacePanel(icon, 320, 480, work)
	if y+480 > icon.Y-PanelGap+0.5 || y < work.Y {
		t.Fatalf("panel at y=%g overlaps the taskbar or leaves the screen", y)
	}
	if x+320 != work.Width-PanelMargin {
		t.Fatalf("panel at x=%g is not held against the right edge", x)
	}
	// A menu bar at the top: the panel opens below it, centered on the icon.
	top := ScreenRect{X: 900, Y: 0, Width: 24, Height: 24}
	work = ScreenRect{Y: 24, Width: 1920, Height: 1056}
	x, y = PlacePanel(top, 300, 400, work)
	if y != top.Height+PanelGap {
		t.Fatalf("panel at y=%g is not just under the menu bar", y)
	}
	if x != top.X+top.Width/2-150 {
		t.Fatalf("panel at x=%g is not centered on its icon", x)
	}
}

func TestPlacePanelStaysInsideATinyArea(t *testing.T) {
	area := ScreenRect{Width: 200, Height: 100}
	x, y := PlacePanel(ScreenRect{X: 100, Y: 90}, 500, 500, area)
	if x != area.X+PanelMargin || y != area.Y {
		t.Fatalf("an oversized panel went to %g,%g instead of the area's corner", x, y)
	}
}

func TestPlacePanelPlacesABareClickPosition(t *testing.T) {
	x, y := PlacePanel(ScreenRect{X: 1000, Y: 1070}, 300, 400, ScreenRect{Width: 1920, Height: 1080})
	if y+400 >= 1070 || x != 850 {
		t.Fatalf("panel for a click at the bottom went to %g,%g", x, y)
	}
}

func TestScaledAveragesByAlpha(t *testing.T) {
	// A 2x2 icon: opaque red, opaque blue, transparent green, half-transparent white.
	icon := StatusIcon{Size: 2, RGBA: []byte{
		255, 0, 0, 255, 0, 0, 255, 255,
		0, 255, 0, 0, 255, 255, 255, 128,
	}, Template: true}
	same := icon.Scaled(2)
	for index, value := range icon.RGBA {
		// A transparent pixel keeps no color of its own.
		if icon.RGBA[index/4*4+3] == 0 && index%4 != 3 {
			value = 0
		}
		if same.RGBA[index] != value {
			t.Fatalf("same-size result = %v, want the source", same.RGBA)
		}
	}
	if !same.Template {
		t.Fatal("scaling dropped the template flag")
	}
	one := icon.Scaled(1)
	if one.Size != 1 || len(one.RGBA) != 4 {
		t.Fatalf("one-pixel icon = %+v", one)
	}
	// Alpha is the mean of 255, 255, 0, 128. The transparent green pixel carries no
	// weight, so the color is that of the others: no green at all.
	if one.RGBA[3] != 160 || one.RGBA[1] > 70 {
		t.Fatalf("averaged pixel = %v", one.RGBA)
	}
	if got := (StatusIcon{}).Scaled(4); got.Size != 0 || got.RGBA != nil {
		t.Fatalf("invalid icon scaled to %+v", got)
	}
	if got := icon.Scaled(0); got.Size != 0 {
		t.Fatalf("zero size gave %+v", got)
	}
}

func TestScaledEnlargesWithoutLosingTheColor(t *testing.T) {
	icon := StatusIcon{Size: 1, RGBA: []byte{10, 20, 30, 255}}
	large := icon.Scaled(4)
	for index := 0; index < len(large.RGBA); index += 4 {
		if got := large.RGBA[index : index+4]; got[0] != 10 || got[1] != 20 || got[2] != 30 || got[3] != 255 {
			t.Fatalf("pixel %d = %v", index/4, got)
		}
	}
}

func BenchmarkScaled(b *testing.B) {
	icon := StatusIcon{Size: 64, RGBA: make([]byte, 64*64*4)}
	b.ReportAllocs()
	for b.Loop() {
		_ = icon.Scaled(16)
	}
}
