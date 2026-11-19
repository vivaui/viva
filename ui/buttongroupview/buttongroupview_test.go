package buttongroupview_test

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/buttongroupview"
)

var (
	leftID   = gfx.Item(201)
	centerID = gfx.Item(202)
	rightID  = gfx.Item(203)
)

type model struct {
	group    buttongroupview.Group
	selected int
	width    float32
	vertical bool
	disabled bool

	clicks       int
	clickedIndex int
	clickedID    gfx.ID
}

func newModel() *model {
	m := &model{selected: -1, width: 270}
	m.group.OnClick(func(index int, id gfx.ID) {
		m.clicks++
		m.clickedIndex, m.clickedID = index, id
		m.selected = index
	})
	return m
}

func (m *model) view(c ui.C) {
	group := &m.group
	group.Reset()
	if m.vertical {
		group.Vertical()
	}
	if m.disabled {
		group.Disabled()
	}
	group.Item(leftID, "Left").Outline()
	group.Item(centerID, "Center").Outline()
	group.Item(rightID, "Right").Outline()
	if m.selected >= 0 {
		group.Selected(m.selected)
	}
	group.View(c, "Text alignment").Width(m.width)
}

func theme() gfx.Theme { return gfx.LightTheme() }

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	palette := theme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &palette})
	canvas := new(gfx.Canvas)
	draw(page, canvas, view)
	return page, canvas
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 420, Height: 220}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

// actions returns the hit regions of the group's actions, without the group's
// own semantic region.
func actions(canvas *gfx.Canvas) []gfx.Hit {
	var result []gfx.Hit
	for _, hit := range canvas.Hits() {
		if hit.Container || hit.Block || hit.ID.IsZero() {
			continue
		}
		result = append(result, hit)
	}
	return result
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

func center(rect gfx.Rect) (float32, float32) {
	return rect.X + rect.Width/2, rect.Y + rect.Height/2
}

func contains(rect gfx.Rect, x, y float32) bool {
	return x >= rect.X && x < rect.X+rect.Width && y >= rect.Y && y < rect.Y+rect.Height
}

func hasColorAt(canvas *gfx.Canvas, color gfx.Color, x, y float32) bool {
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color && contains(rectangle.Bounds, x, y) {
			return true
		}
	}
	return false
}

func hasColor(canvas *gfx.Canvas, color gfx.Color) bool {
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color {
			return true
		}
	}
	return false
}

// The actions are one joined control: their hit regions are contiguous, only
// the group's outer ends are rounded, the middle is square, and the shared
// edges carry the separators instead of independent borders.
func TestJoinedGeometry(t *testing.T) {
	m := newModel()
	_, canvas := open(t, m.view)
	hits := actions(canvas)
	if len(hits) != 3 {
		t.Fatalf("action hits = %d, want 3", len(hits))
	}
	for index := 1; index < len(hits); index++ {
		if !near(hits[index-1].Bounds.X+hits[index-1].Bounds.Width, hits[index].Bounds.X) {
			t.Fatalf("action %d does not share its edge with action %d: %+v", index, index-1, hits)
		}
		if hits[index].Bounds.Width <= 0 || !near(hits[index].Bounds.Height, theme().Metrics.ControlHeight) {
			t.Fatalf("action %d bounds = %+v", index, hits[index].Bounds)
		}
	}

	roundedClipped := 0
	middleSquare := false
	middleX, middleY := center(hits[1].Bounds)
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.HasClip && rectangle.Radius > 0 {
			roundedClipped++
		}
		if !rectangle.HasClip && rectangle.Radius == 0 && contains(rectangle.Bounds, middleX, middleY) {
			middleSquare = true
		}
		// An independent button would paint its own rounded outline over its
		// full hit rectangle. A joined group rounds only its outer corners.
		if !rectangle.HasClip && rectangle.Radius > 0 {
			for _, hit := range hits {
				if near(rectangle.Bounds.X, hit.Bounds.X) && near(rectangle.Bounds.Y, hit.Bounds.Y) &&
					near(rectangle.Bounds.Width, hit.Bounds.Width) && near(rectangle.Bounds.Height, hit.Bounds.Height) {
					t.Fatalf("action at %+v kept an independent rounded surface", hit.Bounds)
				}
			}
		}
	}
	if roundedClipped != 2 {
		t.Fatalf("rounded clipped action surfaces = %d, want the two group ends", roundedClipped)
	}
	if !middleSquare {
		t.Fatal("the middle action is not a square shared surface")
	}
	for index := 1; index < len(hits); index++ {
		if !hasVerticalSeparator(canvas, hits[index].Bounds.X) {
			t.Fatalf("no shared border at action %d's leading edge (%v)", index, hits[index].Bounds.X)
		}
	}
}

func hasVerticalSeparator(canvas *gfx.Canvas, boundary float32) bool {
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color.Alpha == 0 || !near(rectangle.Bounds.Width, 1) || rectangle.Bounds.Height <= 0 {
			continue
		}
		if rectangle.Bounds.X <= boundary && boundary <= rectangle.Bounds.X+rectangle.Bounds.Width {
			return true
		}
	}
	return false
}

func hasHorizontalSeparator(canvas *gfx.Canvas, boundary float32) bool {
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color.Alpha == 0 || !near(rectangle.Bounds.Height, 1) || rectangle.Bounds.Width <= 0 {
			continue
		}
		if rectangle.Bounds.Y <= boundary && boundary <= rectangle.Bounds.Y+rectangle.Bounds.Height {
			return true
		}
	}
	return false
}

// A click reports the action it chose, and the page's Selected paints it with
// the secondary treatment on the next paint.
func TestClickSelects(t *testing.T) {
	m := newModel()
	view := m.view
	page, canvas := open(t, view)
	hits := actions(canvas)

	drawClick(page, canvas, view, hits[1].ID)
	if m.clicks != 1 || m.clickedIndex != 1 || m.clickedID != centerID {
		t.Fatalf("click reported (%d, %v) after %d calls, want action 1 (%v)", m.clickedIndex, m.clickedID, m.clicks, centerID)
	}
	if index, id, ok := m.group.Clicked(); !ok || index != 1 || id != centerID {
		t.Fatalf("Clicked() = (%d, %v, %v), want action 1", index, id, ok)
	}

	draw(page, canvas, view)
	if _, _, ok := m.group.Clicked(); ok {
		t.Fatal("Clicked() stayed true in an ordinary paint")
	}
	if m.clicks != 1 {
		t.Fatalf("the callback ran %d times, want once", m.clicks)
	}

	hits = actions(canvas)
	x, y := center(hits[1].Bounds)
	if !hasColorAt(canvas, theme().Secondary, x, y) {
		t.Fatal("the clicked action was not painted as the selection")
	}
	leftX, leftY := center(hits[0].Bounds)
	if hasColorAt(canvas, theme().Secondary, leftX, leftY) {
		t.Fatal("an unselected action was painted as the selection")
	}
}

func TestSelectedPaintsOnlyTheChosenAction(t *testing.T) {
	m := newModel()
	view := m.view
	page, canvas := open(t, view)
	if hasColor(canvas, theme().Secondary) {
		t.Fatal("an unselected group painted the secondary treatment")
	}

	m.selected = 1
	draw(page, canvas, view)
	hits := actions(canvas)
	x, y := center(hits[1].Bounds)
	if !hasColorAt(canvas, theme().Secondary, x, y) {
		t.Fatal("the selected action was not painted with the secondary treatment")
	}
	leftX, leftY := center(hits[0].Bounds)
	if hasColorAt(canvas, theme().Secondary, leftX, leftY) {
		t.Fatal("an unselected action was painted with the secondary treatment")
	}
}

func TestDisabledIsInert(t *testing.T) {
	m := newModel()
	m.disabled = true
	view := m.view
	page, canvas := open(t, view)
	hits := actions(canvas)
	if len(hits) != 3 {
		t.Fatalf("action hits = %d, want 3", len(hits))
	}
	for _, hit := range hits {
		if !hit.Disabled || hit.Focusable {
			t.Fatalf("disabled action hit = %+v, want inert and out of the Tab order", hit)
		}
	}

	drawClick(page, canvas, view, hits[0].ID)
	if m.clicks != 0 {
		t.Fatalf("the disabled group ran its callback %d times", m.clicks)
	}
	if _, _, ok := m.group.Clicked(); ok {
		t.Fatal("the disabled group reported a click")
	}
}

func TestVerticalStacksJoinedActions(t *testing.T) {
	m := newModel()
	m.vertical = true
	m.width = 120
	_, canvas := open(t, m.view)
	hits := actions(canvas)
	if len(hits) != 3 {
		t.Fatalf("action hits = %d, want 3", len(hits))
	}
	for index := 1; index < len(hits); index++ {
		if !near(hits[index-1].Bounds.Y+hits[index-1].Bounds.Height, hits[index].Bounds.Y) {
			t.Fatalf("action %d does not share its edge with action %d: %+v", index, index-1, hits)
		}
		if !near(hits[index].Bounds.Width, hits[0].Bounds.Width) {
			t.Fatalf("vertical action %d width = %v, want %v", index, hits[index].Bounds.Width, hits[0].Bounds.Width)
		}
		if !near(hits[index].Bounds.Height, theme().Metrics.ControlHeight) {
			t.Fatalf("vertical action %d height = %v", index, hits[index].Bounds.Height)
		}
	}
	roundedClipped := 0
	middleSquare := false
	middleX, middleY := center(hits[1].Bounds)
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.HasClip && rectangle.Radius > 0 {
			roundedClipped++
		}
		if !rectangle.HasClip && rectangle.Radius == 0 && contains(rectangle.Bounds, middleX, middleY) {
			middleSquare = true
		}
	}
	if roundedClipped != 2 {
		t.Fatalf("rounded clipped action surfaces = %d, want the two group ends", roundedClipped)
	}
	if !middleSquare {
		t.Fatal("the middle action is not a square shared surface")
	}
	for index := 1; index < len(hits); index++ {
		if !hasHorizontalSeparator(canvas, hits[index].Bounds.Y) {
			t.Fatalf("no shared border at action %d's leading edge (%v)", index, hits[index].Bounds.Y)
		}
	}
}

// Each action is a focusable button and the group region is semantic only, so
// Tab visits the actions and never a dead group stop.
func TestActionsAreTheTabStops(t *testing.T) {
	m := newModel()
	_, canvas := open(t, m.view)
	hits := actions(canvas)
	if len(hits) != 3 {
		t.Fatalf("action hits = %d, want 3", len(hits))
	}
	for _, hit := range hits {
		if hit.Role != gfx.RoleButton || !hit.Focusable || hit.SkipTab {
			t.Fatalf("action hit = %+v, want a focusable Tab stop with the button role", hit)
		}
	}
	groups := 0
	for _, hit := range canvas.Hits() {
		if !hit.Container {
			continue
		}
		groups++
		if hit.Role != gfx.RoleGroup || !hit.ID.IsZero() || hit.Focusable {
			t.Fatalf("group region = %+v, want a non-focusable group with the view label", hit)
		}
	}
	if groups != 1 {
		t.Fatalf("semantic group regions = %d, want 1", groups)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := newModel()
	view := m.view
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := newModel()
	view := m.view
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
