package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// frameClock is a settable clock for animated test canvases.
type frameClock struct{ now time.Duration }

func newAnimatedCanvas() (*gfx.Canvas, *frameClock) {
	clock := &frameClock{}
	canvas := &gfx.Canvas{}
	canvas.SetClock(func() time.Duration { return clock.now })
	return canvas, clock
}

// paintSwitchFrame paints one switch inside an animated frame at time now.
func paintSwitchFrame(canvas *gfx.Canvas, clock *frameClock, theme gfx.Theme, now time.Duration, id uint32, checked bool) {
	clock.now = now
	canvas.BeginFrame()
	canvas.Reset()
	BeginSwitch(canvas, theme, gfx.Item(int(id)), gfx.Rect{X: 20, Y: 20}, checked).Paint()
	canvas.EndFrame()
}

func TestSwitchSlidesThumbAndFadesTrackBetweenStates(t *testing.T) {
	theme := gfx.LightTheme()
	canvas, clock := newAnimatedCanvas()
	var off, on gfx.Canvas
	BeginSwitch(&off, theme, gfx.Item(1), gfx.Rect{X: 20, Y: 20}, false).Paint()
	BeginSwitch(&on, theme, gfx.Item(1), gfx.Rect{X: 20, Y: 20}, true).Paint()
	offTrack, offThumb := off.Rectangles()[0], off.Rectangles()[1]
	onTrack, onThumb := on.Rectangles()[0], on.Rectangles()[1]

	paintSwitchFrame(canvas, clock, theme, 0, 1, false)
	if canvas.Animating() {
		t.Fatal("a resting switch must not request frames")
	}

	start := time.Second
	paintSwitchFrame(canvas, clock, theme, start, 1, true)
	if !canvas.Animating() || canvas.Rectangles()[0] != offTrack || canvas.Rectangles()[1] != offThumb {
		t.Fatalf("first frame after the change must still show the old state: animating=%v %+v", canvas.Animating(), canvas.Rectangles())
	}
	if !canvas.Hits()[0].Checked {
		t.Fatal("accessibility state must report the destination immediately")
	}

	paintSwitchFrame(canvas, clock, theme, start+theme.Transition.Duration/2, 1, true)
	track, thumb := canvas.Rectangles()[0], canvas.Rectangles()[1]
	if !canvas.Animating() {
		t.Fatal("switch should still be in flight at half duration")
	}
	if thumb.Bounds.X <= offThumb.Bounds.X || thumb.Bounds.X >= onThumb.Bounds.X {
		t.Fatalf("thumb x %v must lie strictly between %v and %v", thumb.Bounds.X, offThumb.Bounds.X, onThumb.Bounds.X)
	}
	if track.Color == offTrack.Color || track.Color == onTrack.Color {
		t.Fatalf("track color %+v must lie between %+v and %+v", track.Color, offTrack.Color, onTrack.Color)
	}
	if thumb.Bounds.Y != offThumb.Bounds.Y || thumb.Bounds.Width != offThumb.Bounds.Width {
		t.Fatalf("thumb must only translate: %+v vs %+v", thumb.Bounds, offThumb.Bounds)
	}

	paintSwitchFrame(canvas, clock, theme, start+theme.Transition.Duration, 1, true)
	if canvas.Animating() {
		t.Fatal("switch must settle at the end of its transition")
	}
	if canvas.Rectangles()[0] != onTrack || canvas.Rectangles()[1] != onThumb {
		t.Fatalf("settled frame must equal the static checked paint: %+v", canvas.Rectangles())
	}
}

func TestSwitchReversalContinuesFromCurrentThumb(t *testing.T) {
	theme := gfx.LightTheme()
	canvas, clock := newAnimatedCanvas()
	paintSwitchFrame(canvas, clock, theme, 0, 1, false)
	paintSwitchFrame(canvas, clock, theme, time.Second, 1, true)
	paintSwitchFrame(canvas, clock, theme, time.Second+theme.Transition.Duration/2, 1, true)
	mid := canvas.Rectangles()[1].Bounds.X
	paintSwitchFrame(canvas, clock, theme, time.Second+theme.Transition.Duration/2, 1, false)
	if got := canvas.Rectangles()[1].Bounds.X; got != mid {
		t.Fatalf("reversing mid-flight jumped the thumb from %v to %v", mid, got)
	}
}

func TestSwitchWithoutMotionPaintsFinalStateImmediately(t *testing.T) {
	staticTheme := gfx.LightTheme()
	staticTheme.Transition = gfx.Motion{}
	canvas, clock := newAnimatedCanvas()
	paintSwitchFrame(canvas, clock, staticTheme, 0, 1, false)
	paintSwitchFrame(canvas, clock, staticTheme, time.Second, 1, true)
	if canvas.Animating() {
		t.Fatal("a theme without transition tokens must stay instant")
	}

	preview := gfx.LightTheme()
	previewCanvas, previewClock := newAnimatedCanvas()
	paintSwitchFrame(previewCanvas, previewClock, preview, 0, 0, false)
	paintSwitchFrame(previewCanvas, previewClock, preview, time.Second, 0, true)
	if previewCanvas.Animating() {
		t.Fatal("static previews (ID 0) must never animate")
	}
}

func TestSwitchDisabledAlphaAppliesToBlendedColors(t *testing.T) {
	theme := gfx.LightTheme()
	canvas, clock := newAnimatedCanvas()
	paintSwitchFrame(canvas, clock, theme, 0, 1, false)
	clock.now = time.Second
	canvas.BeginFrame()
	canvas.Reset()
	BeginSwitch(canvas, theme, gfx.Item(1), gfx.Rect{X: 20, Y: 20}, true).Disabled().Paint()
	canvas.EndFrame()
	if got := canvas.Rectangles()[0].Color.Alpha; got != 0.5 {
		t.Fatalf("disabled switch track alpha at the start of a transition = %v, want 0.5", got)
	}
}

func TestSwitchAnimatedFramesDoNotAllocate(t *testing.T) {
	theme := gfx.DarkTheme()
	canvas, clock := newAnimatedCanvas()
	paintSwitchFrame(canvas, clock, theme, 0, 1, false)
	now := time.Duration(0)
	checked := false
	allocations := testing.AllocsPerRun(200, func() {
		now += 8 * time.Millisecond
		if now%(160*time.Millisecond) == 0 {
			checked = !checked
		}
		paintSwitchFrame(canvas, clock, theme, now, 1, checked)
	})
	if allocations != 0 {
		t.Fatalf("animated switch frames allocated %v times", allocations)
	}
}

func BenchmarkSwitchAnimatedFrame(b *testing.B) {
	theme := gfx.DarkTheme()
	canvas, clock := newAnimatedCanvas()
	paintSwitchFrame(canvas, clock, theme, 0, 1, false)
	b.ReportAllocs()
	b.ResetTimer()
	now := time.Duration(0)
	checked := false
	for range b.N {
		now += 8 * time.Millisecond
		if now%(160*time.Millisecond) == 0 {
			checked = !checked
		}
		paintSwitchFrame(canvas, clock, theme, now, 1, checked)
	}
}
