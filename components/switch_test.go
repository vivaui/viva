package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSwitchOfficialDefaultAndSmallGeometry(t *testing.T) {
	theme := gfx.DarkTheme()
	var defaultCanvas, smallCanvas gfx.Canvas
	BeginSwitch(&defaultCanvas, theme, gfx.ID{}, gfx.Rect{}, false).Paint()
	BeginSwitch(&smallCanvas, theme, gfx.ID{}, gfx.Rect{}, false).Small().Paint()
	defaultTrack := defaultCanvas.Rectangles()[0].Bounds
	smallTrack := smallCanvas.Rectangles()[0].Bounds
	if defaultTrack.Width != 32 || defaultTrack.Height != 18.4 {
		t.Fatalf("default switch geometry = %+v", defaultTrack)
	}
	if smallTrack.Width != 24 || smallTrack.Height != 14 {
		t.Fatalf("small switch geometry = %+v", smallTrack)
	}
	if defaultCanvas.Rectangles()[1].Bounds.Width != 16 || smallCanvas.Rectangles()[1].Bounds.Width != 12 {
		t.Fatalf("thumb sizes = %v / %v", defaultCanvas.Rectangles()[1].Bounds.Width, smallCanvas.Rectangles()[1].Bounds.Width)
	}
}

func TestSwitchCheckedPaintAndTypedState(t *testing.T) {
	theme := gfx.DarkTheme()
	var off, on gfx.Canvas
	BeginSwitch(&off, theme, gfx.Item(1), gfx.Rect{X: 20, Y: 20}, false).Label("Notifications").Paint()
	BeginSwitch(&on, theme, gfx.Item(1), gfx.Rect{X: 20, Y: 20}, true).Label("Notifications").Paint()
	if on.Rectangles()[0].Color != theme.Primary {
		t.Fatalf("checked track = %+v, want Primary", on.Rectangles()[0].Color)
	}
	if on.Rectangles()[1].Bounds.X <= off.Rectangles()[1].Bounds.X {
		t.Fatalf("checked thumb did not move: %+v / %+v", off.Rectangles()[1].Bounds, on.Rectangles()[1].Bounds)
	}
	hit := on.Hits()[0]
	if hit.Role != gfx.RoleSwitch || !hit.Checked || hit.Label != "Notifications" || !hit.Focusable {
		t.Fatalf("typed switch hit = %+v", hit)
	}
	if hit.Bounds != (gfx.Rect{X: 8, Y: 12, Width: 56, Height: 34.4}) {
		t.Fatalf("switch hit bounds = %+v", hit.Bounds)
	}
}

func TestSwitchAppearanceSpecificThumbAndTrack(t *testing.T) {
	light := gfx.LightTheme()
	dark := gfx.DarkTheme()
	var lightCanvas, darkCanvas gfx.Canvas
	BeginSwitch(&lightCanvas, light, gfx.ID{}, gfx.Rect{}, false).Paint()
	BeginSwitch(&darkCanvas, dark, gfx.ID{}, gfx.Rect{}, false).Paint()
	if lightCanvas.Rectangles()[1].Color != light.Background {
		t.Fatalf("light thumb = %+v, want Background", lightCanvas.Rectangles()[1].Color)
	}
	if darkCanvas.Rectangles()[1].Color != dark.Foreground {
		t.Fatalf("dark unchecked thumb = %+v, want Foreground", darkCanvas.Rectangles()[1].Color)
	}
	wantTrack := dark.Input
	wantTrack.Alpha *= 0.80
	if darkCanvas.Rectangles()[0].Color != wantTrack {
		t.Fatalf("dark unchecked track = %+v, want %+v", darkCanvas.Rectangles()[0].Color, wantTrack)
	}
}

func TestSwitchDisabledRetainsSemanticHitAtHalfOpacity(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginSwitch(&canvas, theme, gfx.Item(1), gfx.Rect{}, true).Disabled().Paint()
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || !canvas.Hits()[0].Checked {
		t.Fatalf("disabled switch hit = %+v", canvas.Hits())
	}
	want := theme.Primary
	want.Alpha *= 0.50
	if canvas.Rectangles()[0].Color != want {
		t.Fatalf("disabled track = %+v, want %+v", canvas.Rectangles()[0].Color, want)
	}
}

func TestSwitchFocusUsesOfficialRingAndPreviewHasNoHit(t *testing.T) {
	theme := gfx.DarkTheme()
	var focused, preview gfx.Canvas
	focused.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(1), true)
	BeginSwitch(&focused, theme, gfx.Item(1), gfx.Rect{X: 4, Y: 4}, false).Paint()
	BeginSwitch(&preview, theme, gfx.ID{}, gfx.Rect{}, false).Paint()
	want := theme.Ring
	want.Alpha *= 0.50
	if ring := focused.Rectangles()[0]; ring.Color != want || ring.Bounds != (gfx.Rect{X: 1, Y: 1, Width: 38, Height: 24.4}) {
		t.Fatalf("focus ring = %+v", ring)
	}
	if len(preview.Hits()) != 0 {
		t.Fatalf("preview switch declared %d hits", len(preview.Hits()))
	}
}

func TestSwitchRepeatedPaintReusesCanvasStorage(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginSwitch(&canvas, theme, gfx.Item(1), gfx.Rect{}, true).Paint()
	allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		BeginSwitch(&canvas, theme, gfx.Item(1), gfx.Rect{}, true).Paint()
	})
	if allocations != 0 {
		t.Fatalf("repeated switch paint allocated %v times", allocations)
	}
}

// The thumb rides inside the track's 1px border, so it is one pixel from the
// track's end in both positions, not flush on one side and two pixels off the other.
func TestSwitchThumbIsInsetEvenlyAtBothEnds(t *testing.T) {
	theme := gfx.LightTheme()
	for _, checked := range []bool{false, true} {
		var canvas gfx.Canvas
		BeginSwitch(&canvas, theme, gfx.ID{}, gfx.Rect{X: 10, Y: 10}, checked).Paint()
		track, thumb := canvas.Rectangles()[0].Bounds, canvas.Rectangles()[1].Bounds
		leading, trailing := thumb.X-track.X, track.X+track.Width-(thumb.X+thumb.Width)
		if (checked && trailing != 1) || (!checked && leading != 1) {
			t.Fatalf("checked=%v: thumb %+v in track %+v is %v from the left and %v from the right, want 1 at the end it rests on", checked, thumb, track, leading, trailing)
		}
	}
}

func TestSwitchInvalidPaintsTheDangerRingAndBorder(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	BeginSwitch(&canvas, theme, gfx.Item(1), gfx.Rect{}, false).Invalid().Paint()
	if canvas.Rectangles()[0].Color != invalidRing(&theme) || canvas.Rectangles()[1].Color != invalidBorder(&theme) {
		t.Fatalf("invalid switch rectangles = %+v", canvas.Rectangles())
	}
}
