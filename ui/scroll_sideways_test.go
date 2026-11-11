package ui

import (
	"math"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// wideArea is a page 100 by 100 that is all one ScrollArea, declared by area and
// holding what children declares.
type wideArea struct {
	view     View
	area     func(Box) Box
	children func(f *Frame)
}

func (p *wideArea) paint(canvas *gfx.Canvas, _ gfx.Size) {
	p.view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		f.Column().Fill().Children(func() {
			p.area(f.ScrollArea(listID).Grow(1)).Children(func() { p.children(f) })
		})
	})
}

func (p *wideArea) pointer(canvas *gfx.Canvas, target gfx.ID, kind gfx.PointerEventKind, x, y float32) bool {
	event := gfx.PointerEvent{Kind: kind, Button: gfx.PointerButtonPrimary, X: x, Y: y, Target: target, Captured: kind != gfx.PointerDown}
	return canvas.DeliverPointer(event, gfx.Size{Width: 100, Height: 100}, p.paint)
}

func sideways(b Box) Box { return b.Sideways() }

func TestASidewaysAreaLaysItsChildrenOutAcrossItsContent(t *testing.T) {
	var wide, stretched, fixed Box
	p := &wideArea{area: sideways, children: func(f *Frame) {
		wide = f.Slot().Width(300).Height(20)
		stretched = f.Slot().Height(20)
		fixed = f.Slot().Width(40).Height(20)
	}}
	canvas := testCanvas()
	p.paint(canvas, gfx.Size{})
	state := p.view.ScrollState(listID)
	if state.ContentWidth != 300 || state.ViewportWidth != 100 || state.MaxX() != 200 {
		t.Fatalf("content %v wide in %v, max %v; want 300 in 100, max 200", state.ContentWidth, state.ViewportWidth, state.MaxX())
	}
	if got := stretched.Rect(); got.X != 0 || got.Width != 300 {
		t.Errorf("a child that stretches is %+v, want it across the content, x 0 width 300", got)
	}
	if got := fixed.Rect(); got.Width != 40 {
		t.Errorf("a child with a width is %v wide, want 40", got.Width)
	}

	state.ScrollToX(50)
	p.paint(canvas, gfx.Size{})
	for name, b := range map[string]Box{"wide": wide, "stretched": stretched, "fixed": fixed} {
		if got := b.Rect().X; got != -50 {
			t.Errorf("after ScrollToX(50) the %s child is at x %v, want -50", name, got)
		}
	}

	state.ScrollToX(1000) // past the end: the layout keeps it in range
	p.paint(canvas, gfx.Size{})
	if wide.Rect().X != -200 || state.OffsetX != 200 {
		t.Errorf("after ScrollToX(1000) the content is at x %v and OffsetX %v, want -200 and 200", wide.Rect().X, state.OffsetX)
	}
}

// A sideways area is as wide as it is given, as tall as it is given, so content wider
// than the window does not push its neighbors; with a MaxWidth it is as wide as its
// content up to that.
func TestASidewaysAreaIsAsWideAsItIsGiven(t *testing.T) {
	var view View
	var area, after Box
	draw := func(declare func(Box) Box) {
		f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100})
		f.Row().Fill().Children(func() {
			area = declare(f.ScrollArea(listID).Sideways()).Children(func() { f.Slot().Width(500).Height(20) })
			after = f.Slot().Width(50)
		})
		f.End()
	}
	draw(func(b Box) Box { return b.Grow(1) })
	if area.Rect().Width != 150 || after.Rect().X != 150 {
		t.Errorf("with Grow the area is %v wide and its neighbor at %v, want 150 and 150", area.Rect().Width, after.Rect().X)
	}
	draw(func(b Box) Box { return b.MaxWidth(120) })
	if area.Rect().Width != 120 || after.Rect().X != 120 {
		t.Errorf("with MaxWidth(120) the area is %v wide and its neighbor at %v, want 120 and 120", area.Rect().Width, after.Rect().X)
	}
	draw(func(b Box) Box { return b })
	if area.Rect().Width != 0 || after.Rect().X != 0 {
		t.Errorf("with no width the area is %v wide and its neighbor at %v, want 0 and 0", area.Rect().Width, after.Rect().X)
	}
}

// Without Sideways an area stays as it was: no horizontal offset, and Shift with the
// wheel still scrolls it down.
func TestAnAreaThatIsNotSidewaysDoesNotScrollAcross(t *testing.T) {
	var wide Box
	p := &wideArea{area: func(b Box) Box { return b }, children: func(f *Frame) {
		wide = f.Slot().Width(300).Height(300)
	}}
	canvas := testCanvas()
	p.paint(canvas, gfx.Size{})
	state := p.view.ScrollState(listID)
	state.ScrollToX(50)
	state.Wheel(gfx.ScrollEvent{DeltaX: 30, Precise: true})
	p.paint(canvas, gfx.Size{})
	if wide.Rect().X != 0 || state.OffsetX != 0 || state.ContentWidth != state.ViewportWidth {
		t.Errorf("content at x %v, OffsetX %v, content %v in %v; want 0, 0 and the viewport's width",
			wide.Rect().X, state.OffsetX, state.ContentWidth, state.ViewportWidth)
	}
	state.Wheel(gfx.ScrollEvent{DeltaY: 30, Precise: true, Modifiers: gfx.ModifierShift})
	if state.Offset != 30 {
		t.Errorf("Shift with the wheel moved the area to %v, want 30 down as before", state.Offset)
	}
}

func TestTheWheelMovesASidewaysAreaAcross(t *testing.T) {
	p := &wideArea{area: sideways, children: func(f *Frame) {
		for range 10 {
			f.Slot().Width(300).Height(30)
		}
	}}
	canvas := testCanvas()
	p.paint(canvas, gfx.Size{})
	state := p.view.ScrollState(listID)
	steps := []struct {
		name       string
		event      gfx.ScrollEvent
		x, y       float32
		repainting bool
	}{
		{"a trackpad's horizontal delta", gfx.ScrollEvent{DeltaX: 30, Precise: true}, 30, 0, true},
		{"a notch with Shift", gfx.ScrollEvent{DeltaY: 1, Modifiers: gfx.ModifierShift}, 54, 0, true},
		{"a vertical delta", gfx.ScrollEvent{DeltaY: 10, Precise: true}, 54, 10, true},
		{"both at once", gfx.ScrollEvent{DeltaX: -4, DeltaY: 5, Precise: true}, 50, 15, true},
		{"past the right end", gfx.ScrollEvent{DeltaX: 500, Precise: true}, 200, 15, true},
		{"at the right end", gfx.ScrollEvent{DeltaX: 5, Precise: true}, 200, 15, false},
	}
	for _, step := range steps {
		result := state.Wheel(step.event)
		if state.OffsetX != step.x || state.Offset != step.y || result.Repaint != step.repainting {
			t.Errorf("%s: offset %v across, %v down, repaint %v; want %v, %v, %v",
				step.name, state.OffsetX, state.Offset, result.Repaint, step.x, step.y, step.repainting)
		}
	}
}

// The View gives a sideways wheel to the sideways area under the pointer, and a wheel
// down past it to the area around it, which is what a row of tabs above a page needs.
func TestAViewGivesASidewaysWheelToTheAreaUnderThePointer(t *testing.T) {
	var view View
	strip, page := tree.ID(page, "strip"), tree.ID(page, "page")
	draw := func() {
		f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100})
		f.Column().Fill().Children(func() {
			f.ScrollArea(page).Grow(1).Children(func() {
				f.ScrollArea(strip).Height(30).Sideways().Children(func() {
					f.Row().Children(func() {
						for range 5 {
							f.Slot().Width(100).Height(30)
						}
					})
				})
				for range 10 {
					f.Slot().Height(30)
				}
			})
		})
		f.End()
	}
	draw()
	across := func() float32 { return view.ScrollState(strip).OffsetX }
	down := func() float32 { return view.ScrollState(page).Offset }
	steps := []struct {
		name    string
		event   gfx.ScrollEvent
		x, y    float32
		handled bool
	}{
		{"a horizontal wheel over the strip", gfx.ScrollEvent{X: 10, Y: 15, DeltaX: 40, Precise: true}, 40, 0, true},
		{"Shift with the wheel over the strip", gfx.ScrollEvent{X: 10, Y: 15, DeltaY: 40, Precise: true, Modifiers: gfx.ModifierShift}, 80, 0, true},
		{"a vertical wheel over the strip", gfx.ScrollEvent{X: 10, Y: 15, DeltaY: 10, Precise: true}, 80, 10, true},
		{"a horizontal wheel below the strip", gfx.ScrollEvent{X: 10, Y: 80, DeltaX: 40, Precise: true}, 80, 10, false},
	}
	for _, step := range steps {
		result := view.Wheel(step.event)
		draw()
		if across() != step.x || down() != step.y || result.Handled != step.handled {
			t.Errorf("%s: strip at %v, page at %v, handled %v; want %v, %v, %v",
				step.name, across(), down(), result.Handled, step.x, step.y, step.handled)
		}
	}
}

func TestScrollbarsAreDrawnWhereTheContentOverflows(t *testing.T) {
	var width, height float32
	p := &wideArea{area: func(b Box) Box { return b.Sideways().Scrollbars() }, children: func(f *Frame) {
		f.Slot().Width(width).Height(height)
	}}
	parts := tree.Sub(listID.Scope(), listID.Name())
	vertical, horizontal := parts.At(0), parts.At(1)
	for _, size := range []struct {
		width, height        float32
		vertical, horizontal bool
	}{
		{50, 50, false, false},
		{50, 300, true, false},
		{300, 50, false, true},
		{300, 300, true, true},
	} {
		width, height = size.width, size.height
		canvas := testCanvas()
		p.paint(canvas, gfx.Size{})
		p.paint(canvas, gfx.Size{}) // the first paint lays the content out; the bars follow it
		_, v := canvas.HitBounds(vertical)
		_, h := canvas.HitBounds(horizontal)
		if v != size.vertical || h != size.horizontal {
			t.Errorf("content %vx%v in 100x100: vertical bar %v, horizontal %v; want %v, %v",
				width, height, v, h, size.vertical, size.horizontal)
		}
	}
}

func TestDraggingAScrollbarMovesTheArea(t *testing.T) {
	p := &wideArea{area: func(b Box) Box { return b.Sideways().Scrollbars() }, children: func(f *Frame) {
		f.Slot().Width(300).Height(300)
	}}
	canvas := testCanvas()
	p.paint(canvas, gfx.Size{})
	p.paint(canvas, gfx.Size{})
	state := p.view.ScrollState(listID)
	parts := tree.Sub(listID.Scope(), listID.Name())
	vertical, horizontal := parts.At(0), parts.At(1)
	geometry := components.BeginScrollArea(nil, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, gfx.Size{Width: 300, Height: 300}).
		HoverScrollbars(true).Geometry()
	thumb := geometry.VerticalThumb
	near := func(got, want float32) bool { return math.Abs(float64(got-want)) < 0.01 }

	// Taken by its middle and dragged 20 down, the thumb keeps the pointer where it held it.
	x, y := thumb.X+1, thumb.Y+thumb.Height/2
	if !p.pointer(canvas, vertical, gfx.PointerDown, x, y) || state.Offset != 0 {
		t.Fatalf("a press on the thumb was not taken, or moved the area to %v", state.Offset)
	}
	p.pointer(canvas, vertical, gfx.PointerMove, x, y+20)
	if want := geometry.VerticalOffsetForThumb(thumb.Y + 20); !near(state.Offset, want) || want <= 0 {
		t.Errorf("dragged 20 down the area is at %v, want %v", state.Offset, want)
	}
	if !p.pointer(canvas, vertical, gfx.PointerUp, x, y+20) {
		t.Error("letting go of the thumb was not taken")
	}
	before := state.Offset
	if p.pointer(canvas, vertical, gfx.PointerMove, x, y+60) || state.Offset != before {
		t.Errorf("a move after the release was taken, or moved the area from %v to %v", before, state.Offset)
	}

	// A press at the bottom of the bar, below the thumb, brings the thumb there: the end.
	p.paint(canvas, gfx.Size{})
	p.pointer(canvas, vertical, gfx.PointerDown, x, geometry.VerticalTrack.Y+geometry.VerticalTrack.Height-1)
	p.pointer(canvas, vertical, gfx.PointerUp, x, geometry.VerticalTrack.Y+geometry.VerticalTrack.Height-1)
	if state.Offset != state.Max() {
		t.Errorf("a press at the bottom of the bar moved the area to %v, want its end %v", state.Offset, state.Max())
	}

	// The horizontal bar moves the area across, from where its thumb was taken.
	p.paint(canvas, gfx.Size{})
	thumb = geometry.HorizontalThumb
	x, y = thumb.X+2, thumb.Y+1
	p.pointer(canvas, horizontal, gfx.PointerDown, x, y)
	p.pointer(canvas, horizontal, gfx.PointerMove, x+30, y)
	p.pointer(canvas, horizontal, gfx.PointerUp, x+30, y)
	if want := geometry.HorizontalOffsetForThumb(thumb.X + 30); !near(state.OffsetX, want) || want <= 0 {
		t.Errorf("dragged 30 right the area is at %v across, want %v", state.OffsetX, want)
	}
	if state.Offset != state.Max() {
		t.Errorf("dragging across moved the area down too, to %v", state.Offset)
	}
}

func TestASidewaysAreaWithScrollbarsAllocatesNothingOnceWarm(t *testing.T) {
	p := &wideArea{area: func(b Box) Box { return b.Sideways().Scrollbars() }, children: func(f *Frame) {
		for range 10 {
			f.Slot().Width(300).Height(30)
		}
	}}
	canvas := testCanvas()
	p.paint(canvas, gfx.Size{})
	parts := tree.Sub(listID.Scope(), listID.Name())
	vertical := parts.At(0)
	frame := func() {
		canvas.Reset()
		p.paint(canvas, gfx.Size{})
		p.view.Wheel(gfx.ScrollEvent{X: 5, Y: 5, DeltaX: 3, DeltaY: -3, Precise: true})
		p.pointer(canvas, vertical, gfx.PointerDown, 97, 10)
		p.pointer(canvas, vertical, gfx.PointerUp, 97, 10)
	}
	frame()
	if got := testing.AllocsPerRun(50, frame); got != 0 {
		t.Errorf("%v allocations per frame, want 0", got)
	}
}

func BenchmarkTenSidewaysAreas(b *testing.B) {
	var view View
	canvas := testCanvas()
	ids := make([]gfx.ID, 10)
	for i := range ids {
		ids[i] = gfx.Item(i + 1)
	}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Row().Fill().Children(func() {
				for _, id := range ids {
					f.ScrollArea(id).Width(40).Sideways().Scrollbars().Children(func() { f.Slot().Width(100).Height(500) })
				}
			})
		})
	}
}
