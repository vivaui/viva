package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type dragModel struct {
	state    ui.DragState
	accept   func(source, target gfx.ID) bool
	drop     func(source, target gfx.ID)
	accepted int
	dropped  int
	clicked  int
	hidden   bool
	source   gfx.ID
	target   gfx.ID
}

func newDragModel(acceptTarget bool) *dragModel {
	m := &dragModel{}
	m.accept = func(source, target gfx.ID) bool { m.accepted++; return acceptTarget }
	m.drop = func(source, target gfx.ID) { m.dropped++; m.source, m.target = source, target }
	return m
}

func (m *dragModel) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		if m.hidden {
			return
		}
		source := ui.Row(c).Height(30).FillWidth().Hit(ui.ID(c), "Source").DragSource(&m.state)
		ui.Row(c).Height(30).FillWidth().Hit(ui.ID(c), "Target").DropTarget(&m.state, m.accept, m.drop)
		if source.Clicked() {
			m.clicked++
		}
	})
}

func (m *dragModel) open(t *testing.T) *pageDriver {
	t.Helper()
	return openPage(t, m.page)
}

func testDrag(t *testing.T) (*dragModel, *pageDriver, gfx.Hit, gfx.Hit) {
	t.Helper()
	m := newDragModel(true)
	d := m.open(t)
	source, target := d.hit("Source"), d.hit("Target")
	sx, sy := center(source.Bounds)
	tx, ty := center(target.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.state.Dragging() || !m.state.Source().IsZero() {
		t.Fatal("a press before the threshold is already a drag")
	}
	if err := d.driver.Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	return m, d, source, target
}

func TestDragStateDropsOnAnAcceptedTarget(t *testing.T) {
	m, d, source, target := testDrag(t)
	if !m.state.Dragging() || m.state.Source() != source.ID || m.state.Target() != target.ID {
		t.Fatalf("drag: dragging=%v source=%v target=%v", m.state.Dragging(), m.state.Source(), m.state.Target())
	}
	if at := m.state.At(); at.X != centerX(target.Bounds) || at.Y != centerY(target.Bounds) {
		t.Fatalf("At = %v, not the pointer", at)
	}
	tx, ty := center(target.Bounds)
	if err := d.driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 || m.accepted == 0 || m.source != source.ID || m.target != target.ID {
		t.Fatalf("dropped=%d accepted=%d source=%v target=%v", m.dropped, m.accepted, m.source, m.target)
	}
	if m.state.Dragging() || !m.state.Target().IsZero() {
		t.Fatal("the drop did not end the gesture")
	}
	if m.clicked != 0 {
		t.Fatalf("a drag ended in %d clicks", m.clicked)
	}
}

func TestDragStateLeavesARejectedTargetAndAClickAlone(t *testing.T) {
	m := newDragModel(false)
	d := m.open(t)
	source, target := d.hit("Source"), d.hit("Target")
	sx, sy := center(source.Bounds)
	tx, ty := center(target.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	if !m.state.Dragging() || m.state.Source() != source.ID {
		t.Fatalf("drag: dragging=%v source=%v", m.state.Dragging(), m.state.Source())
	}
	if !m.state.Target().IsZero() || m.accepted == 0 {
		t.Fatalf("rejected target=%v accepted=%d", m.state.Target(), m.accepted)
	}
	if err := d.driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 0 {
		t.Fatalf("a rejected target received %d drops", m.dropped)
	}
	if m.clicked != 0 {
		t.Fatalf("a drag ended in %d clicks", m.clicked)
	}

	if err := d.driver.Click(sx, sy); err != nil {
		t.Fatal(err)
	}
	if m.clicked != 1 {
		t.Fatalf("a press that did not move: %d clicks", m.clicked)
	}
	if m.state.Dragging() {
		t.Fatal("a click left a drag active")
	}
}

func TestDragStateEscapeLetsGoWithoutDropping(t *testing.T) {
	m, d, source, _ := testDrag(t)
	d.key(gfx.KeyEscape)
	if m.state.Dragging() || !m.state.Target().IsZero() {
		t.Fatalf("Escape: dragging=%v target=%v", m.state.Dragging(), m.state.Target())
	}
	sx, sy := center(source.Bounds)
	if err := d.driver.Move(sx, sy); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 0 || m.clicked != 0 {
		t.Fatalf("a cancelled drag dropped %d times and clicked %d times", m.dropped, m.clicked)
	}
}

func TestDragStateTargetHonorsHitOutset(t *testing.T) {
	m := newDragModel(true)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
			ui.Row(c).Height(30).FillWidth().Hit(ui.ID(c), "Source").DragSource(&m.state)
			ui.Row(c).Height(30).FillWidth().Hit(ui.ID(c), "Target").HitOutset(4, 0, 4, 0).
				DropTarget(&m.state, m.accept, m.drop)
		})
	})
	source, target := d.hit("Source"), d.hit("Target")
	sx, sy := center(source.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	x, y := centerX(target.Bounds), target.Bounds.Y+target.Bounds.Height-2
	if err := d.driver.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if m.state.Target() != target.ID {
		t.Fatalf("a target's HitOutset was not part of its drop region: %v", m.state.Target())
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 {
		t.Fatalf("dropped %d", m.dropped)
	}
}

// A paint that no longer declares the drag's elements ends the gesture, so a
// hidden page cannot leave a drag held.
func TestDragStateEndsWhenThePaintDropsIt(t *testing.T) {
	m, d, source, _ := testDrag(t)
	m.hidden = true
	d.repaint()
	if m.state.Dragging() || !m.state.Source().IsZero() || !m.state.Target().IsZero() {
		t.Fatalf("after the paint: dragging=%v source=%v target=%v", m.state.Dragging(), m.state.Source(), m.state.Target())
	}
	sx, sy := center(source.Bounds)
	if err := d.driver.Release(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 0 || m.clicked != 0 {
		t.Fatalf("a drag whose paint dropped it dropped %d times and clicked %d times", m.dropped, m.clicked)
	}
}

// The drag takes the move and release from the window's own pointer handler,
// while the press that may still be a click reaches it.
func TestDragStateTakesThePointerFromBase(t *testing.T) {
	m := newDragModel(true)
	var kinds []gfx.PointerEventKind
	d := openPageWith(t, ui.PageConfig{View: m.page}, app.Config{OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
		kinds = append(kinds, event.Kind)
		return gfx.EventResult{}
	}})
	source, target := d.hit("Source"), d.hit("Target")
	sx, sy := center(source.Bounds)
	tx, ty := center(target.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if len(kinds) == 0 || kinds[len(kinds)-1] != gfx.PointerDown {
		t.Fatalf("base saw %v, want the press last and no move or release of the drag", kinds)
	}
	for _, kind := range kinds {
		if kind == gfx.PointerUp {
			t.Fatalf("base saw %v, the drag's release reached it", kinds)
		}
	}
	if m.dropped != 1 {
		t.Fatalf("dropped %d", m.dropped)
	}
}

// A surface painted through a ui.View inside a page hands the state to the page
// with Drag; the page routes it, without the window's own pointer handler.
func TestDragStateInAPagePaintedView(t *testing.T) {
	var tree gfx.Tree
	feature := tree.Feature("grid")
	sourceID, targetID := tree.ID(feature, "source"), tree.ID(feature, "target")
	m := newDragModel(true)
	var view ui.View
	draw := func(d *ui.DrawContext) {
		view.Draw(d.Canvas, gfx.LightTheme(), d.Rect, func(f *ui.Frame) {
			f.Column().Fill().Pad(16).Gap(8).Children(func() {
				f.Row().Height(30).FillWidth().Hit(sourceID, "Source").DragSource(&m.state)
				f.Row().Height(30).FillWidth().Hit(targetID, "Target").DropTarget(&m.state, m.accept, m.drop)
			})
		})
	}
	d := openPage(t, func(c ui.C) {
		ui.Drag(c, &m.state)
		ui.Column(c).Fill().Children(func() {
			ui.Slot(c).Fill().Draw(draw)
		})
	})
	source, target := d.hit("Source"), d.hit("Target")
	sx, sy := center(source.Bounds)
	tx, ty := center(target.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 || m.source != sourceID || m.target != targetID {
		t.Fatalf("dropped=%d source=%v target=%v", m.dropped, m.source, m.target)
	}
}

// A Frame painted through its own ui.View has no page; its host wires Pointer
// and Key and gets the same gesture.
func TestDragStateInAFrameView(t *testing.T) {
	var tree gfx.Tree
	feature := tree.Feature("drag")
	sourceID, targetID := tree.ID(feature, "source"), tree.ID(feature, "target")
	m := newDragModel(true)
	var view ui.View
	paint := func(canvas *gfx.Canvas, size gfx.Size) {
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, func(f *ui.Frame) {
			f.Column().Fill().Pad(16).Gap(8).Children(func() {
				f.Row().Height(30).FillWidth().Hit(sourceID, "Source").DragSource(&m.state)
				f.Row().Height(30).FillWidth().Hit(targetID, "Target").DropTarget(&m.state, m.accept, m.drop)
			})
		})
	}
	driver, err := app.NewDriver(app.Config{Width: 480, Height: 480, Tree: &tree, Paint: paint,
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			if result, ok := m.state.Pointer(event); ok {
				return result
			}
			return gfx.EventResult{}
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			if result, ok := m.state.Key(event); ok {
				return result
			}
			return gfx.EventResult{}
		}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var source, target gfx.Hit
	for _, hit := range driver.Snapshot().Hits {
		switch hit.Label {
		case "Source":
			source = hit
		case "Target":
			target = hit
		}
	}
	sx, sy := center(source.Bounds)
	tx, ty := center(target.Bounds)
	if err := driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 {
		t.Fatalf("a Frame's drag dropped %d times", m.dropped)
	}
}

// A drag repaints without allocating.
func TestDragRepaintAllocates(t *testing.T) {
	_, d, _, target := testDrag(t)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
	tx, ty := center(target.Bounds)
	if err := d.driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkDragRepaint(b *testing.B) {
	m := newDragModel(true)
	page := ui.NewPage(ui.PageConfig{View: m.page})
	invalidator := new(app.Invalidator)
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 480, Invalidator: invalidator}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	var source, target gfx.Hit
	for _, hit := range driver.Snapshot().Hits {
		switch hit.Label {
		case "Source":
			source = hit
		case "Target":
			target = hit
		}
	}
	if source.ID.IsZero() || target.ID.IsZero() {
		b.Fatal("the drag source or target is not in the snapshot")
	}
	sx, sy := center(source.Bounds)
	tx, ty := center(target.Bounds)
	if err := driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		b.Fatal(err)
	}
	if err := driver.Move(tx, ty); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
}

func centerX(r gfx.Rect) float32 { return r.X + r.Width/2 }
func centerY(r gfx.Rect) float32 { return r.Y + r.Height/2 }
