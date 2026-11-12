package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// partDragModel is a custom board of cards drawn by one Control: every card is a
// part, declared as both a source and a target as the control draws it, and a
// fourth target is only drawn, with no hit of its own.
type partDragModel struct {
	state    ui.DragState
	handlers ui.Handlers
	ids      [4]gfx.ID
	accept   func(source, target gfx.ID) bool
	drop     func(source, target gfx.ID)
	drawFn   func(*ui.DrawContext) // m.draw, made once: a method value made in each paint allocates
	accepted int
	dropped  int
	source   gfx.ID
	target   gfx.ID
}

func newPartDragModel() *partDragModel {
	m := &partDragModel{}
	m.accept = func(source, target gfx.ID) bool { m.accepted++; return true }
	m.drop = func(source, target gfx.ID) { m.dropped++; m.source, m.target = source, target }
	m.drawFn = m.draw
	return m
}

func (m *partDragModel) card(d *ui.DrawContext, i int) gfx.Rect {
	return gfx.Rect{X: d.Rect.X + float32(i)*70, Y: d.Rect.Y, Width: 60, Height: 60}
}

// draw declares three cards as hits, sources and targets, and a fourth region as a
// target alone.
func (m *partDragModel) draw(d *ui.DrawContext) {
	parts := m.handlers.Parts()
	for i := range 3 {
		r := m.card(d, i)
		m.ids[i] = parts.At(i)
		d.Canvas.Fill(r, d.Theme.Muted)
		d.Canvas.HitWithOptions(m.ids[i], r, partLabels[i], gfx.HitOptions{Role: gfx.RoleButton})
		d.DragSource(&m.state, m.ids[i], r)
		d.DropTarget(&m.state, m.ids[i], r, m.accept, m.drop)
	}
	r := m.card(d, 3)
	m.ids[3] = parts.At(3)
	d.Canvas.Fill(r, d.Theme.Muted)
	d.DropTarget(&m.state, m.ids[3], r, m.accept, m.drop)
}

var partLabels = [3]string{"Part 0", "Part 1", "Part 2"}

func (m *partDragModel) page(c ui.C) {
	ui.Column(c).Children(func() {
		ui.Control(c, "Board", &m.handlers).Size(300, 64).Draw(m.drawFn)
	})
}

func (m *partDragModel) open(t *testing.T) *pageDriver {
	t.Helper()
	return openPage(t, m.page)
}

// A press on a part of a control starts the control's DragState, the part is the
// source, and the release drops once on the part under the pointer.
func TestPartPressStartsDragAndDropsOnce(t *testing.T) {
	m := newPartDragModel()
	d := m.open(t)
	p0, p1 := d.hit("Part 0"), d.hit("Part 1")
	if m.ids[0].IsZero() || m.ids[1].IsZero() || m.ids[0] == m.ids[1] {
		t.Fatalf("parts have no distinct identities: %v %v", m.ids[0], m.ids[1])
	}
	sx, sy := center(p0.Bounds)
	tx, ty := center(p1.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.state.Dragging() || !m.state.Source().IsZero() {
		t.Fatalf("a press on a part is already a drag: dragging=%v source=%v", m.state.Dragging(), m.state.Source())
	}
	if err := d.driver.Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	if !m.state.Dragging() || m.state.Source() != m.ids[0] || m.state.Target() != m.ids[1] {
		t.Fatalf("dragging=%v source=%v target=%v, want %v -> %v",
			m.state.Dragging(), m.state.Source(), m.state.Target(), m.ids[0], m.ids[1])
	}
	if err := d.driver.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 || m.source != m.ids[0] || m.target != m.ids[1] {
		t.Fatalf("dropped=%d source=%v target=%v", m.dropped, m.source, m.target)
	}
	if m.state.Dragging() || !m.state.Target().IsZero() || !m.state.Source().IsZero() {
		t.Fatalf("the drop did not end the gesture: dragging=%v source=%v target=%v",
			m.state.Dragging(), m.state.Source(), m.state.Target())
	}
}

// accept is asked with the pressed part and each part under the pointer, and only
// a part it takes becomes Target.
func TestPartTargetHonorsAccept(t *testing.T) {
	m := newPartDragModel()
	m.accept = func(source, target gfx.ID) bool {
		m.accepted++
		return source == m.ids[0] && target == m.ids[1]
	}
	d := m.open(t)
	p0, p1, p2 := d.hit("Part 0"), d.hit("Part 1"), d.hit("Part 2")
	sx, sy := center(p0.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	x, y := center(p2.Bounds)
	if err := d.driver.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if !m.state.Target().IsZero() {
		t.Fatalf("a rejected part is the target: %v", m.state.Target())
	}
	if m.accepted == 0 {
		t.Fatal("accept was not asked")
	}
	x, y = center(p1.Bounds)
	if err := d.driver.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if m.state.Target() != m.ids[1] {
		t.Fatalf("target = %v, want the accepted part %v", m.state.Target(), m.ids[1])
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 || m.source != m.ids[0] || m.target != m.ids[1] {
		t.Fatalf("dropped=%d source=%v target=%v", m.dropped, m.source, m.target)
	}
}

// A target part needs no Canvas.Hit: its drawn rectangle is what the pointer is
// tested against.
func TestPartTargetWithoutAHitTakesTheDrop(t *testing.T) {
	m := newPartDragModel()
	d := m.open(t)
	p0 := d.hit("Part 0")
	board := d.hit("Board")
	sx, sy := center(p0.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	x := board.Bounds.X + 230
	y := board.Bounds.Y + board.Bounds.Height/2
	if err := d.driver.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if m.state.Target() != m.ids[3] {
		t.Fatalf("target = %v, want the hitless part %v", m.state.Target(), m.ids[3])
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 1 || m.target != m.ids[3] {
		t.Fatalf("dropped=%d target=%v, want one drop on %v", m.dropped, m.target, m.ids[3])
	}
}

// Escape lets go of a part drag without a drop and without leaving a click.
func TestPartDragEscapeCancels(t *testing.T) {
	m := newPartDragModel()
	d := m.open(t)
	p0, p1 := d.hit("Part 0"), d.hit("Part 1")
	sx, sy := center(p0.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	x, y := center(p1.Bounds)
	if err := d.driver.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if !m.state.Dragging() {
		t.Fatal("the part drag did not start")
	}
	d.key(gfx.KeyEscape)
	if m.state.Dragging() || !m.state.Target().IsZero() || !m.state.Source().IsZero() {
		t.Fatalf("Escape: dragging=%v source=%v target=%v", m.state.Dragging(), m.state.Source(), m.state.Target())
	}
	if err := d.driver.Move(sx, sy); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.dropped != 0 {
		t.Fatalf("a cancelled part drag dropped %d times", m.dropped)
	}
}

// Declaring a part drag needs a page's Draw: a DrawContext that is not one panics.
func TestPartDragOutsideAPageDrawPanics(t *testing.T) {
	var tree gfx.Tree
	id := tree.ID(tree.Feature("board"), "card")
	t.Run("zero context", func(t *testing.T) {
		var state ui.DragState
		var d ui.DrawContext
		defer func() {
			if recover() == nil {
				t.Fatal("a part drag declared outside a page Draw did not panic")
			}
		}()
		d.DragSource(&state, id, gfx.Rect{Width: 10, Height: 10})
	})
	t.Run("view surface", func(t *testing.T) {
		m := newPartDragModel()
		var view ui.View
		page := func(c ui.C) {
			ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) {
				view.Draw(d.Canvas, *d.Theme, d.Rect, func(f *ui.Frame) {
					f.Row().Size(10, 10).Draw(func(inner *ui.DrawContext) {
						inner.DragSource(&m.state, id, gfx.Rect{Width: 10, Height: 10})
					})
				})
			})
		}
		defer func() {
			if recover() == nil {
				t.Fatal("a part drag declared in a ui.View surface did not panic")
			}
		}()
		openPage(t, page)
	})
}

// Declaring part drags as the control draws allocates nothing.
func TestPartDragRepaintAllocates(t *testing.T) {
	m := newPartDragModel()
	d := m.open(t)
	p0, p1 := d.hit("Part 0"), d.hit("Part 1")
	sx, sy := center(p0.Bounds)
	if err := d.driver.Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	x, y := center(p1.Bounds)
	if err := d.driver.Move(x, y); err != nil {
		t.Fatal(err)
	}
	if !m.state.Dragging() {
		t.Fatal("the part drag did not start")
	}
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
}
