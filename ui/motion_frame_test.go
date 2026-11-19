package ui_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A page Switch animates exactly as the components.Switch it wraps: the page
// path only gives the component its leaf, so the thumb's travel is the same
// rectangle on every frame of the transition.
func TestPageSwitchAnimatesLikeItsComponent(t *testing.T) {
	theme := gfx.LightTheme()
	on := false
	pageClock := newParityClock()
	view := func(c ui.C) { ui.Switch(c, &on, "") }
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	pagePaint := func() { page.Draw(pageClock.canvas, gfx.Rect{Width: 200, Height: 100}, view) }
	pageClock.step(false, pagePaint)
	// The first rectangle of the page's resting frame is the track: the page
	// centers the component's switch inside the Box's enlarged hit region.
	track := pageClock.canvas.Rectangles()[0].Bounds

	componentClock := newParityClock()
	componentPaint := func() {
		components.BeginSwitch(componentClock.canvas, theme, gfx.Item(7), track, on).Paint()
	}
	// Settle both at off before the toggle, so the transition starts from the
	// same look on both paths.
	componentClock.step(false, componentPaint)

	// Turn it on, then sample every frame until both rest.
	on = true
	for frame := 0; ; frame++ {
		pageClock.step(frame > 0, pagePaint)
		componentClock.step(frame > 0, componentPaint)
		sameRects(t, "switch", frame, componentClock.parityRects(), pageClock.parityRects())
		if pageClock.settled() != componentClock.settled() {
			t.Fatalf("switch frame %d: components at rest=%v, page at rest=%v", frame, componentClock.settled(), pageClock.settled())
		}
		if pageClock.settled() {
			if frame == 0 {
				t.Fatal("the switch settled on its first frame; its thumb did not travel")
			}
			return
		}
		if frame > 100 {
			t.Fatal("the switch transition never settled")
		}
	}
}

// Hovering a page control paints its hover look at once: without a Transition
// the control schedules no frame and a later paint is the same picture, so a
// hand-painted hover and a page one cannot drift apart over time.
func TestHoverPaintsAtOnceAndSchedulesNoFrames(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Pad(60).Children(func() { ui.Button(c, "Save") })
	})
	before := d.driver.Snapshot().Primitives
	hit := d.hit("Save")
	if err := d.driver.Move(center(hit.Bounds)); err != nil {
		t.Fatal(err)
	}
	hovered := d.driver.Snapshot().Primitives
	if reflect.DeepEqual(before, hovered) {
		t.Fatal("moving onto the button changed nothing; the test is not hovering it")
	}
	for _, step := range []time.Duration{16 * time.Millisecond, 32 * time.Millisecond, 150 * time.Millisecond} {
		if err := d.driver.Advance(step); err != nil {
			t.Fatal(err)
		}
		if d.driver.Transitioning() {
			t.Fatalf("a hover started a transition after %v", step)
		}
		if err := d.driver.Paint(); err != nil {
			t.Fatal(err)
		}
		if now := d.driver.Snapshot().Primitives; !reflect.DeepEqual(hovered, now) {
			t.Fatalf("the hover look changed after %v without a Transition", step)
		}
	}
}

// A wheel scroll moves the content once: the frame after the wheel shows the new
// offset, and later frames are the same picture, so scrolling is not an
// animation with intermediate positions.
func TestScrollMovesAtOnceAndSchedulesNoFrames(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Scroll(c).Fill().AlwaysScrollbars().Children(func() {
			ui.Column(c).Children(func() { ui.Slot(c).Height(1000) })
		})
	})
	before := d.driver.Snapshot().Primitives
	if err := d.driver.Scroll(gfx.ScrollEvent{X: 100, Y: 100, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	scrolled := d.driver.Snapshot().Primitives
	if reflect.DeepEqual(before, scrolled) {
		t.Fatal("the wheel changed nothing; the test is not scrolling the area")
	}
	for _, step := range []time.Duration{16 * time.Millisecond, 32 * time.Millisecond} {
		if err := d.driver.Advance(step); err != nil {
			t.Fatal(err)
		}
		if d.driver.Transitioning() {
			t.Fatalf("a scroll started a transition after %v", step)
		}
		if err := d.driver.Paint(); err != nil {
			t.Fatal(err)
		}
		if now := d.driver.Snapshot().Primitives; !reflect.DeepEqual(scrolled, now) {
			t.Fatalf("the scrolled picture changed after %v; scrolling animated", step)
		}
	}
}
