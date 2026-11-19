package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func namedRangeHit(t *testing.T, d *pageDriver, id gfx.ID) []gfx.Hit {
	t.Helper()
	var hits []gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.ID == id {
			hits = append(hits, hit)
		}
	}
	if len(hits) == 0 {
		t.Fatalf("no hit named %v", id)
	}
	return hits
}

// A caller names the four parts of a page RangeSlider, and the page still routes
// presses on them to the slider: dragging the named low thumb moves its value.
func TestRangeSliderCallerNamedPartsRoutePresses(t *testing.T) {
	state := ui.NewRangeSliderState(20, 80)
	state.LowID, state.HighID = gfx.Item(101), gfx.Item(102)
	state.TrackID, state.ValueID = gfx.Item(103), gfx.Item(104)
	state.LowLabel, state.HighLabel = "Low price", "High price"
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	})
	low := namedRangeHit(t, d, state.LowID)[0]
	x, y := center(low.Bounds)
	if err := d.driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(x+40, y); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(x+40, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if state.Low <= 20 || state.Low >= state.High {
		t.Fatalf("low=%v high=%v after dragging the caller-named thumb", state.Low, state.High)
	}
	namedRangeHit(t, d, state.HighID)
	namedRangeHit(t, d, state.TrackID)
	namedRangeHit(t, d, state.ValueID)
}

// A press on the caller-named track moves the nearer thumb there, so the track
// is more than an identity: it is a part the page routes.
func TestRangeSliderTrackPressMovesTheNearerThumb(t *testing.T) {
	state := ui.NewRangeSliderState(20, 80)
	state.LowID, state.HighID = gfx.Item(201), gfx.Item(202)
	state.TrackID, state.ValueID = gfx.Item(203), gfx.Item(204)
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	})
	segments := namedRangeHit(t, d, state.TrackID)
	left := segments[0]
	for _, segment := range segments[1:] {
		if segment.Bounds.X < left.Bounds.X {
			left = segment
		}
	}
	if left.Bounds.Width < 2 {
		t.Fatalf("leftmost track segment is %+v, too narrow to press", left.Bounds)
	}
	if err := d.driver.Press(left.Bounds.X+1, left.Bounds.Y+left.Bounds.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(left.Bounds.X+1, left.Bounds.Y+left.Bounds.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if state.Low >= 20 {
		t.Fatalf("low=%v after a press at the start of the track, want it moved down from 20", state.Low)
	}
}

// The thumbs' enlarged hit regions stay inside the control's own group region
// at both ends of the range, so the control's area never overlaps a part it does
// not contain.
func TestRangeSliderThumbsStayInsideTheControlAtTheEnds(t *testing.T) {
	state := ui.NewRangeSliderState(0, 100)
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	})
	control := d.hit("Range").Bounds
	for _, label := range []string{"Minimum", "Maximum"} {
		thumb := d.hit(label).Bounds
		if thumb.X < control.X || thumb.Y < control.Y ||
			thumb.X+thumb.Width > control.X+control.Width ||
			thumb.Y+thumb.Height > control.Y+control.Height {
			t.Fatalf("%s hit %+v reaches outside the control %+v", label, thumb, control)
		}
	}
}

// Naming the parts adds aliases and hit regions, not per-frame work: a warm
// named range slider paints without allocating.
func TestRangeSliderNamedPartsAllocateNothingOnceWarm(t *testing.T) {
	state := ui.NewRangeSliderState(25, 80)
	state.LowID, state.HighID = gfx.Item(301), gfx.Item(302)
	state.TrackID, state.ValueID = gfx.Item(303), gfx.Item(304)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	canvas := benchCanvas()
	view := func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	}
	draw := func() { drawPage(page, canvas, view) }
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("a warm named range slider allocates %v times a frame, want 0", allocations)
	}
}
