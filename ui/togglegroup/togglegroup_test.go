package togglegroup_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/togglegroup"
)

func measure(value string, size, weight float32) float32 {
	return float32(len(value)) * size * 0.55
}

func drawGroup(t *testing.T, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	redraw(page, canvas, view)
	return page, canvas
}

func redraw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 320, Height: 160}, view)
	canvas.EndFrame()
}

func clickItem(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	redraw(page, canvas, view)
	canvas.EndEventPass()
	redraw(page, canvas, view)
}

func hitByLabel(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].IsLabel && !hits[i].Backdrop {
			return hits[i]
		}
	}
	t.Fatalf("no hit labeled %q", label)
	return gfx.Hit{}
}

func itemHits(t *testing.T, canvas *gfx.Canvas, labels ...string) []gfx.Hit {
	t.Helper()
	hits := make([]gfx.Hit, len(labels))
	for index, label := range labels {
		hits[index] = hitByLabel(t, canvas, label)
	}
	return hits
}

func rectangleAt(canvas *gfx.Canvas, bounds gfx.Rect) (gfx.Rectangle, bool) {
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds == bounds {
			return rectangle, true
		}
	}
	return gfx.Rectangle{}, false
}

// hasRoundedClippedSurface reports a rounded surface clipped at its item, as
// the outer items of a joined group are painted.
func hasRoundedClippedSurface(canvas *gfx.Canvas, item gfx.Rect) bool {
	x, y := item.X+item.Width/2, item.Y+item.Height/2
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Radius == 0 || !rectangle.HasClip {
			continue
		}
		b := rectangle.Bounds
		if x >= b.X && x <= b.X+b.Width && y >= b.Y && y <= b.Y+b.Height {
			return true
		}
	}
	return false
}

// joinedSurface is the part of item index's rectangle the component paints on:
// a framed group's border takes the outer edge.
func joinedSurface(bounds gfx.Rect, index, count int) gfx.Rect {
	bounds.Y, bounds.Height = bounds.Y+1, bounds.Height-2
	if index == 0 {
		bounds.X, bounds.Width = bounds.X+1, bounds.Width-1
	}
	if index == count-1 {
		bounds.Width--
	}
	return bounds
}

// Joined outline items share their edges and only the outer corners are
// rounded, matching components.BeginToggleGroup.
func TestJoinedOutlineSharesEdgesAndRoundsOnlyOuterCorners(t *testing.T) {
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Outline()
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	_, canvas := drawGroup(t, view)
	hits := itemHits(t, canvas, "Left", "Center", "Right")
	if hitByLabel(t, canvas, "Text alignment").Focusable {
		t.Fatal("the group container is an extra tab stop")
	}
	for _, hit := range hits {
		if !hit.Focusable {
			t.Fatalf("item %q is not focusable", hit.Label)
		}
	}
	for index := 0; index < len(hits)-1; index++ {
		next := hits[index+1].Bounds
		edge := hits[index].Bounds.X + hits[index].Bounds.Width
		if edge != next.X {
			t.Fatalf("items %d and %d do not share an edge: %v and %v", index, index+1, hits[index].Bounds, next)
		}
	}
	if !hasRoundedClippedSurface(canvas, hits[0].Bounds) {
		t.Fatal("the first item surface is not rounded and clipped at the group's outer corner")
	}
	middle, ok := rectangleAt(canvas, joinedSurface(hits[1].Bounds, 1, 3))
	if !ok || middle.Radius != 0 || middle.HasClip {
		t.Fatalf("middle item surface = %+v, found %v", middle, ok)
	}
	if !hasRoundedClippedSurface(canvas, hits[2].Bounds) {
		t.Fatal("the last item surface is not rounded and clipped at the group's outer corner")
	}
}

func TestVerticalItemsShareHorizontalEdges(t *testing.T) {
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Vertical().Outline()
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	_, canvas := drawGroup(t, view)
	hits := itemHits(t, canvas, "Left", "Center", "Right")
	for index := 0; index < len(hits)-1; index++ {
		edge := hits[index].Bounds.Y + hits[index].Bounds.Height
		if edge != hits[index+1].Bounds.Y {
			t.Fatalf("items %d and %d do not share a horizontal edge: %v and %v", index, index+1, hits[index].Bounds, hits[index+1].Bounds)
		}
		if hits[index].Bounds.X != hits[index+1].Bounds.X || hits[index].Bounds.Width != hits[index+1].Bounds.Width {
			t.Fatalf("vertical items %d and %d have different columns", index, index+1)
		}
	}
}

func TestSingleChoiceWritesTheCallerIndex(t *testing.T) {
	selected := 0
	var changed []int
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Single(&selected).Outline().
		OnChange(func(index int, active bool) { changed = append(changed, index) })
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	page, canvas := drawGroup(t, view)
	if !hitByLabel(t, canvas, "Left").Checked {
		t.Fatal("the initial selection is not painted")
	}
	right := hitByLabel(t, canvas, "Right").ID
	clickItem(page, canvas, view, right)
	if selected != 2 {
		t.Fatalf("selected = %d, want 2", selected)
	}
	if len(changed) != 1 || changed[0] != 2 {
		t.Fatalf("OnChange calls = %v, want [2]", changed)
	}
	if !hitByLabel(t, canvas, "Right").Checked || hitByLabel(t, canvas, "Left").Checked {
		t.Fatal("the clicked item did not become the selection")
	}
	// Activating the selected item again does not report a change.
	clickItem(page, canvas, view, right)
	if len(changed) != 1 {
		t.Fatalf("OnChange calls after re-activation = %v", changed)
	}
}

func TestMultipleChoiceTogglesTheCallerBits(t *testing.T) {
	active := make([]bool, 3)
	var changed []int
	var group togglegroup.Group
	group.Items("Bold", "Italic", "Underline").Multiple(active).Outline().
		OnChange(func(index int, isActive bool) {
			if isActive {
				changed = append(changed, index)
			} else {
				changed = append(changed, -index)
			}
		})
	view := func(c ui.C) { group.View(c, "Text styles").Width(120) }
	page, canvas := drawGroup(t, view)
	bold := hitByLabel(t, canvas, "Bold").ID
	italic := hitByLabel(t, canvas, "Italic").ID
	clickItem(page, canvas, view, bold)
	clickItem(page, canvas, view, italic)
	if !active[0] || !active[1] || active[2] {
		t.Fatalf("active = %v, want [true true false]", active)
	}
	if !hitByLabel(t, canvas, "Bold").Checked || !hitByLabel(t, canvas, "Italic").Checked {
		t.Fatal("the toggled items are not painted active")
	}
	clickItem(page, canvas, view, bold)
	if active[0] || !active[1] {
		t.Fatalf("active after un-toggling = %v, want [false true false]", active)
	}
	want := []int{0, 1, -0}
	if len(changed) != len(want) {
		t.Fatalf("OnChange calls = %v, want %v", changed, want)
	}
	for index := range want {
		if changed[index] != want[index] {
			t.Fatalf("OnChange calls = %v, want %v", changed, want)
		}
	}
}

func TestDisabledItemAndGroupAreInert(t *testing.T) {
	selected := 0
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Single(&selected).Disable(1).Outline()
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	page, canvas := drawGroup(t, view)
	center := hitByLabel(t, canvas, "Center")
	if !center.Disabled {
		t.Fatal("the disabled item does not report its state")
	}
	clickItem(page, canvas, view, center.ID)
	if selected != 0 {
		t.Fatalf("a disabled item changed the selection to %d", selected)
	}
	key := page.Key(gfx.KeyEvent{Target: hitByLabel(t, canvas, "Left").ID, Key: gfx.KeyRight})
	if !key.Handled || key.Focus != hitByLabel(t, canvas, "Right").ID {
		t.Fatalf("Right from the first item should skip the disabled one: %+v", key)
	}

	var locked togglegroup.Group
	lockedSelected := 0
	locked.Items("Left", "Center", "Right").Single(&lockedSelected).Outline().Disabled()
	lockedView := func(c ui.C) { locked.View(c, "Locked alignment").Width(120) }
	lockedPage, lockedCanvas := drawGroup(t, lockedView)
	for _, label := range []string{"Left", "Center", "Right"} {
		if !hitByLabel(t, lockedCanvas, label).Disabled {
			t.Fatalf("%s is not disabled in a disabled group", label)
		}
	}
	clickItem(lockedPage, lockedCanvas, lockedView, hitByLabel(t, lockedCanvas, "Center").ID)
	if lockedSelected != 0 {
		t.Fatalf("a disabled group changed the selection to %d", lockedSelected)
	}
}

func TestKeyboardMovesFocusAndActivates(t *testing.T) {
	selected := 0
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Single(&selected).Outline()
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	page, canvas := drawGroup(t, view)
	left := hitByLabel(t, canvas, "Left").ID
	center := hitByLabel(t, canvas, "Center").ID

	result := page.Key(gfx.KeyEvent{Target: left, Key: gfx.KeyRight})
	if !result.Handled || result.Focus != center || !result.FocusVisible {
		t.Fatalf("Right: %+v, want focus on Center", result)
	}
	if selected != 1 {
		t.Fatalf("single-choice Right selected %d, want 1", selected)
	}
	if !result.Repaint {
		t.Fatal("a selection change on Right did not request a repaint")
	}

	result = page.Key(gfx.KeyEvent{Target: center, Key: gfx.KeyEnd})
	if !result.Handled || result.Focus != hitByLabel(t, canvas, "Right").ID || selected != 2 {
		t.Fatalf("End: %+v selected=%d", result, selected)
	}
	result = page.Key(gfx.KeyEvent{Target: hitByLabel(t, canvas, "Right").ID, Key: gfx.KeySpace})
	if !result.Handled || result.Repaint {
		t.Fatalf("Space on the selected item = %+v", result)
	}
}

func TestMultipleKeyboardMovesFocusWithoutToggling(t *testing.T) {
	active := make([]bool, 3)
	var group togglegroup.Group
	group.Items("Bold", "Italic", "Underline").Multiple(active).Outline()
	view := func(c ui.C) { group.View(c, "Text styles").Width(120) }
	page, canvas := drawGroup(t, view)
	bold := hitByLabel(t, canvas, "Bold").ID
	italic := hitByLabel(t, canvas, "Italic").ID
	result := page.Key(gfx.KeyEvent{Target: bold, Key: gfx.KeyRight})
	if !result.Handled || result.Focus != italic || result.Repaint {
		t.Fatalf("Right: %+v, want focus on Italic without a repaint", result)
	}
	if active[0] || active[1] {
		t.Fatalf("an arrow toggled bits: %v", active)
	}
	result = page.Key(gfx.KeyEvent{Target: italic, Key: gfx.KeySpace})
	if !result.Handled || !result.Repaint || !active[1] {
		t.Fatalf("Space: %+v active=%v", result, active)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	selected := 0
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Single(&selected).Outline()
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	page, canvas := drawGroup(t, view)
	paint := func() { redraw(page, canvas, view) }
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("warm paint allocations = %v, want 0", allocations)
	}
}

func BenchmarkView(b *testing.B) {
	selected := 0
	var group togglegroup.Group
	group.Items("Left", "Center", "Right").Single(&selected).Outline()
	view := func(c ui.C) { group.View(c, "Text alignment").Width(120) }
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	redraw(page, canvas, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		redraw(page, canvas, view)
	}
}
