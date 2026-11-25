package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// parityClock is a canvas whose animated frames advance on a test clock, and
// the paint pass a test drives frame by frame.
type parityClock struct {
	canvas *gfx.Canvas
	now    time.Duration
}

func newParityClock() *parityClock {
	clock := &parityClock{canvas: &gfx.Canvas{}}
	clock.canvas.SetClock(func() time.Duration { return clock.now })
	clock.canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len(value)) * size / 2
	})
	return clock
}

// step advances time by one display frame and paints one pass.
func (clock *parityClock) step(advance bool, paint func()) {
	if advance {
		clock.now += 16 * time.Millisecond
	}
	clock.canvas.Reset()
	clock.canvas.BeginFrame()
	paint()
	clock.canvas.EndFrame()
}

func (clock *parityClock) settled() bool {
	_, next := clock.canvas.NextFrame()
	return !next
}

// parityRects copies the frame's rectangles with the layer left out: a page
// paints overlays on its own layer, which is not part of what the motion is.
func (clock *parityClock) parityRects() []gfx.Rectangle {
	rects := append([]gfx.Rectangle(nil), clock.canvas.Rectangles()...)
	for i := range rects {
		rects[i].Layer = 0
	}
	return rects
}

func sameRects(t *testing.T, label string, frame int, old, new []gfx.Rectangle) {
	t.Helper()
	if len(old) != len(new) {
		t.Fatalf("%s frame %d: components painted %d rectangles, the page %d:\ncomponents %+v\npage       %+v", label, frame, len(old), len(new), old, new)
	}
	for i := range old {
		if old[i] != new[i] {
			t.Fatalf("%s frame %d: rectangle %d is %+v on the components path and %+v on the page path", label, frame, i, old[i], new[i])
		}
	}
}

// A page Popover enters and leaves exactly as the components.Popover it
// replaces: the same rectangles, at the same places with the same opacity, on
// every frame of the entrance and of the exit. The autofill and completion
// offers, drawn by components before the page migration, therefore animate
// as they did.
func TestPagePopoverAnimatesLikeTheComponentsPopover(t *testing.T) {
	theme := gfx.LightTheme()
	anchor := gfx.Rect{X: 100, Y: 100, Width: 40, Height: 20}
	const width = 200
	const bodyHeight = 120
	// The page's panel is the body plus its border and padding; the component
	// is given the panel's height directly.
	panelHeight := bodyHeight + 2*(1+theme.Metrics.PadSurface)

	oldClock := newParityClock()
	oldPaint := func(open bool) func() {
		return func() {
			if !open {
				return
			}
			popover := components.BeginPopover(oldClock.canvas, theme, gfx.Rect{Width: 480, Height: 480}, anchor, components.PopoverOptions{
				DismissID: gfx.Item(99), Side: components.PopoverBottom, Align: components.PopoverAlignStart,
				SideOffset: 2, Headerless: true, ContentSize: gfx.Size{Width: width, Height: panelHeight},
			})
			popover.Begin("", "")
			popover.End()
		}
	}

	open := true
	newClock := newParityClock()
	view := func(c ui.C) {
		slot := ui.Slot(c).Absolute().Left(anchor.X).Top(anchor.Y).Size(anchor.Width, anchor.Height)
		ui.Popover(c, slot, &open, ui.PopoverSpec{
			Scope: gfx.Item(7), Side: components.PopoverBottom, Align: ui.Start, Offset: 2, Width: width,
		}, func() {
			ui.Slot(c).FillWidth().Height(bodyHeight)
		})
	}
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	newPaint := func() {
		page.Draw(newClock.canvas, gfx.Rect{Width: 480, Height: 480}, view)
	}

	// Open: the entrance runs until both paths rest.
	for frame := 0; ; frame++ {
		oldClock.step(frame > 0, oldPaint(true))
		newClock.step(frame > 0, newPaint)
		sameRects(t, "entrance", frame, oldClock.parityRects(), newClock.parityRects())
		if oldSettled, newSettled := oldClock.settled(), newClock.settled(); oldSettled != newSettled {
			t.Fatalf("entrance frame %d: components at rest=%v, page at rest=%v", frame, oldSettled, newSettled)
		}
		if oldClock.settled() {
			if frame == 0 {
				t.Fatal("the entrance settled on its first frame; nothing was animated")
			}
			break
		}
		if frame > 100 {
			t.Fatal("the entrance never settled")
		}
	}

	// Close: the leaving panel is replayed from the last frame each path painted.
	open = false
	for frame := 0; ; frame++ {
		oldClock.step(true, oldPaint(false))
		newClock.step(true, newPaint)
		sameRects(t, "exit", frame, oldClock.parityRects(), newClock.parityRects())
		if oldClock.settled() && newClock.settled() {
			break
		}
		if frame > 100 {
			t.Fatal("the exit never settled")
		}
	}
	if rects := oldClock.parityRects(); len(rects) != 0 {
		t.Fatalf("the components popover left %d rectangles behind", len(rects))
	}
	if rects := newClock.parityRects(); len(rects) != 0 {
		t.Fatalf("the page popover left %d rectangles behind", len(rects))
	}
}

// Instant is the one difference kept from the components options: a list that
// appears under the cursor as the user types has no entrance, so the page
// paints it at its place on the first frame and asks for no next frame.
func TestPagePopoverInstantAppearsAtOnce(t *testing.T) {
	theme := gfx.LightTheme()
	open := true
	clock := newParityClock()
	view := func(c ui.C) {
		slot := ui.Slot(c).Absolute().Left(100).Top(100).Size(40, 20)
		ui.Popover(c, slot, &open, ui.PopoverSpec{
			Scope: gfx.Item(7), Side: components.PopoverBottom, Align: ui.Start, Offset: 2, Width: 200, Instant: true,
		}, func() {
			ui.Slot(c).FillWidth().Height(120)
		})
	}
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	paint := func() { page.Draw(clock.canvas, gfx.Rect{Width: 480, Height: 480}, view) }
	clock.step(false, paint)
	if !clock.settled() {
		t.Fatal("an instant popover asked for another frame")
	}
	rects := clock.parityRects()
	if len(rects) == 0 || rects[0].Bounds.Y != 122 {
		t.Fatalf("an instant popover's panel is at %+v, want it at its resting place already", rects)
	}
}

// A page Drawer enters and leaves exactly as components.BeginDrawer does, with
// the assistant panel's inset and nonmodal spec: the scrim's fade and the
// panel's slide match on every frame of both transitions.
func TestPageDrawerAnimatesLikeTheComponentsDrawer(t *testing.T) {
	theme := gfx.LightTheme()
	frame := gfx.Rect{Width: 480, Height: 480}
	inset := gfx.Insets{Top: 30}
	viewport := frame.Inset(inset)
	const size = 300

	oldClock := newParityClock()
	oldPaint := func(open bool) func() {
		return func() {
			if !open {
				return
			}
			drawer := components.BeginDrawer(oldClock.canvas, theme, viewport, components.DrawerOptions{
				DismissID: gfx.Item(99), Side: components.DrawerRight, ContentExtent: size,
			})
			drawer.Begin("", "")
			drawer.End()
		}
	}

	open := true
	newClock := newParityClock()
	view := func(c ui.C) {
		ui.Drawer(c, &open, "", ui.DrawerSpec{Side: ui.DrawerRight, Size: size, Inset: inset, NonModal: true}, func() {
			ui.Slot(c).FillWidth().Height(40)
		}, nil)
	}
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	newPaint := func() { page.Draw(newClock.canvas, frame, view) }

	for frame := 0; ; frame++ {
		oldClock.step(frame > 0, oldPaint(true))
		newClock.step(frame > 0, newPaint)
		sameRects(t, "drawer entrance", frame, oldClock.parityRects(), newClock.parityRects())
		if oldSettled, newSettled := oldClock.settled(), newClock.settled(); oldSettled != newSettled {
			t.Fatalf("drawer entrance frame %d: components at rest=%v, page at rest=%v", frame, oldSettled, newSettled)
		}
		if oldClock.settled() {
			if frame == 0 {
				t.Fatal("the drawer entrance settled on its first frame; nothing was animated")
			}
			break
		}
		if frame > 100 {
			t.Fatal("the drawer entrance never settled")
		}
	}

	open = false
	for frame := 0; ; frame++ {
		oldClock.step(true, oldPaint(false))
		newClock.step(true, newPaint)
		sameRects(t, "drawer exit", frame, oldClock.parityRects(), newClock.parityRects())
		if oldClock.settled() && newClock.settled() {
			break
		}
		if frame > 100 {
			t.Fatal("the drawer exit never settled")
		}
	}
}
