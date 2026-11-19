package reordergridview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/reordergridview"
)

const (
	testColumns = 3
	testGap     = float32(10)
	testHeight  = float32(60)
)

var testIDs = [6]gfx.ID{
	gfx.Item(1), gfx.Item(2), gfx.Item(3), gfx.Item(4), gfx.Item(5), gfx.Item(6),
}

type model struct {
	view    reordergridview.Grid
	page    *ui.Page
	canvas  *gfx.Canvas
	moves   []reordergridview.Move
	target  gfx.ID
	keys    int
	scrolls int
}

func (m *model) wire() {
	m.view.HandleHitSlop = 8
	m.view.Draw = func(d *ui.DrawContext, layout reordergridview.Layout) {
		for order, id := range testIDs {
			layout.Order(id, 0, order, false)
		}
		for order, id := range testIDs {
			card := layout.ItemBounds(testColumns, order, testGap, testHeight)
			handle := gfx.Rect{X: card.X + 8, Y: card.Y + 12, Width: 28, Height: card.Height - 24}
			layout.Item(id, "Host", 0, order, card, handle, false)
		}
	}
	m.view.Move = func(move reordergridview.Move) { m.moves = append(m.moves, move) }
	m.view.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.target = event.Target
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.view.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.keys++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	m.view.Scroll = func(event gfx.ScrollEvent) gfx.EventResult {
		m.scrolls++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
}

func (m *model) viewPage(c ui.C) {
	reordergridview.View(c, "Hosts", &m.view).Size(600, 220)
}

func (m *model) open(t testing.TB) {
	t.Helper()
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 640, Height: 260}, view)
	canvas.EndFrame()
}

func hitLabelled(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].ID.IsZero() {
			return hits[i], true
		}
	}
	return gfx.Hit{}, false
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	got, ok := hitLabelled(canvas, label)
	if !ok {
		t.Fatalf("no hit labelled %q was painted", label)
	}
	return got
}

// The page declares the view, the caller paints the cells in Draw, and the
// wrapper gives every handle an identity inside the control so pointer and key
// events reach it. Completed moves keep the caller's identities.
func TestViewPaintsAndRoutesReorder(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	if viewport := m.view.Viewport(); viewport.Width != 600 || viewport.Height != 220 {
		t.Fatalf("viewport = %+v, want 600x220", viewport)
	}
	first := m.view.ItemBounds(testColumns, 0, testGap, testHeight)
	last := m.view.ItemBounds(testColumns, 4, testGap, testHeight)
	if first.Width <= 0 || last.Y <= first.Y {
		t.Fatalf("bounds = %+v %+v, want a laid-out grid", first, last)
	}

	hits := m.canvas.Hits()
	var handles []gfx.Hit
	for _, h := range hits {
		if h.Label == "Host" && !h.ID.IsZero() {
			handles = append(handles, h)
		}
	}
	if len(handles) != len(testIDs) {
		t.Fatalf("painted %d handles, want %d", len(handles), len(testIDs))
	}
	for _, h := range handles {
		if h.ID == testIDs[0] || h.ID == testIDs[3] {
			t.Fatal("a handle was painted with the caller's identity instead of the control's")
		}
	}

	from := gfx.Point{X: handles[0].Bounds.X + handles[0].Bounds.Width/2, Y: handles[0].Bounds.Y + handles[0].Bounds.Height/2}
	to := gfx.Point{X: last.X + last.Width/2, Y: last.Y + last.Height/2}
	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: handles[0].ID, X: from.X, Y: from.Y,
	}); !result.Handled || m.target != testIDs[0] {
		t.Fatalf("pointer down Target = %v (result %+v), want the caller's %v", m.target, result, testIDs[0])
	}
	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerMove, Captured: true, Target: handles[0].ID, X: to.X, Y: to.Y,
	}); !result.Handled || m.target != testIDs[0] {
		t.Fatalf("pointer move Target = %v (result %+v)", m.target, result)
	}
	if !m.view.Dragging() || m.view.Target() != testIDs[4] {
		t.Fatalf("dragging = %v target = %v, want %v", m.view.Dragging(), m.view.Target(), testIDs[4])
	}
	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerUp, Captured: true, Target: handles[0].ID, X: to.X, Y: to.Y,
	}); !result.Handled {
		t.Fatalf("pointer up result = %+v", result)
	}
	if len(m.moves) != 1 || m.moves[0].ID != testIDs[0] || m.moves[0].Target != testIDs[4] ||
		m.moves[0].From != 0 || m.moves[0].To != 4 || !m.moves[0].Valid() {
		t.Fatalf("moves = %+v, want a caller-identity move from 0 to 4", m.moves)
	}

	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyEnd, Target: handles[0].ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d calls", result, m.keys)
	}
	if len(m.moves) != 2 || m.moves[1].ID != testIDs[0] || m.moves[1].To != 5 {
		t.Fatalf("moves = %+v, want a keyboard move to the last slot", m.moves)
	}

	if result := m.page.Wheel(gfx.ScrollEvent{Target: handles[0].ID, DeltaY: 1}); !result.Handled || m.scrolls != 1 {
		t.Fatalf("wheel result = %+v after %d calls", result, m.scrolls)
	}
}

// A warm repaint allocates nothing: the wrapper keeps its handlers, draw
// function and item identities, and the component keeps its own geometry.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

// An item with the zero identity is neither painted nor draggable.
func TestZeroItemIdentityIsOmitted(t *testing.T) {
	var view reordergridview.Grid
	view.Draw = func(d *ui.DrawContext, layout reordergridview.Layout) {
		card := layout.ItemBounds(1, 0, 0, 40)
		layout.Item(gfx.ID{}, "Missing", 0, 0, card, card, false)
		layout.Item(gfx.Item(9), "Present", 0, 1, layout.ItemBounds(1, 1, 0, 40), card, false)
	}
	viewPage := func(c ui.C) { reordergridview.View(c, "Hosts", &view).Size(300, 120) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: viewPage, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, viewPage)
	if _, ok := hitLabelled(canvas, "Missing"); ok {
		t.Fatal("an item with the zero identity was painted")
	}
	hit(t, canvas, "Present")
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.open(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
