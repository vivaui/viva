package menubar_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/menubar"
)

var (
	fileID    = gfx.Item(1)
	viewID    = gfx.Item(2)
	newID     = gfx.Item(3)
	gridID    = gfx.Item(4)
	deleteID  = gfx.Item(5)
	reloadID  = gfx.Item(6)
	dismissID = gfx.Item(7)
)

type model struct {
	bar menubar.State

	grid bool

	pointerTarget gfx.ID
	keyTarget     gfx.ID
	keys          int
}

func (m *model) wire() {
	m.bar.Scope = gfx.Item(80)
	m.bar.DismissID = dismissID
	m.bar.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.pointerTarget = event.Target
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.bar.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.keys++
		m.keyTarget = event.Target
		return gfx.EventResult{Handled: true}
	}
}

func (m *model) view(c ui.C) {
	m.bar.Reset()
	m.bar.Trigger(fileID, "File").
		Item(newID, "New window").Shortcut("⌘N").
		Item(gridID, "Show grid").Checkbox(m.grid).
		Separator().
		Item(deleteID, "Delete workspace").Destructive()
	m.bar.Trigger(viewID, "View").Item(reloadID, "Reload")
	m.bar.View(c, "State").Width(420).Height(36)
	if m.bar.Selected == gridID {
		m.grid = !m.grid
		m.bar.Selected = gfx.ID{}
	}
}

func openPage(t *testing.T, m *model) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	m.wire()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.view, Theme: &theme})
	canvas := benchCanvas()
	draw(page, canvas, m.view)
	return page, canvas
}

func benchCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	return canvas
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 600, Height: 400}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

func hitLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.Label == label && !hit.ID.IsZero() && !hit.Backdrop {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func mustHit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hit, ok := hitLabel(canvas, label)
	if !ok {
		t.Fatalf("no hit labelled %q", label)
	}
	return hit
}

func TestMenubarOpensFromATriggerAndChoosesARow(t *testing.T) {
	m := &model{}
	page, canvas := openPage(t, m)

	drawClick(page, canvas, m.view, fileID)
	if m.bar.Open != fileID {
		t.Fatalf("open = %v, want File", m.bar.Open)
	}
	draw(page, canvas, m.view)
	for _, label := range []string{"New window", "Show grid", "Delete workspace"} {
		mustHit(t, canvas, label)
	}

	// A click the runtime delivers on the row's internal identity is translated
	// back to the caller's identity.
	row := mustHit(t, canvas, "New window")
	drawClick(page, canvas, m.view, row.ID)
	if m.bar.Selected != newID {
		t.Fatalf("selected = %v, want the New window row", m.bar.Selected)
	}
	if !m.bar.Open.IsZero() {
		t.Fatal("choosing an action row left the menu open")
	}

	// The caller identity works too, as direct event passes and tests do.
	m.bar.Open = fileID
	drawClick(page, canvas, m.view, deleteID)
	if m.bar.Selected != deleteID {
		t.Fatalf("selected = %v, want the Delete workspace row", m.bar.Selected)
	}
}

func TestMenubarTogglesACheckboxRow(t *testing.T) {
	m := &model{}
	m.bar.Open = fileID
	page, canvas := openPage(t, m)
	if m.grid {
		t.Fatal("grid starts checked")
	}
	drawClick(page, canvas, m.view, gridID)
	if m.bar.Selected != gridID {
		t.Fatalf("selected = %v, want the Show grid row", m.bar.Selected)
	}
	// The caller owns the checked state: apply the reported choice and repaint.
	m.grid, m.bar.Selected = true, gfx.ID{}
	draw(page, canvas, m.view)
	row := mustHit(t, canvas, "Show grid")
	if !row.Checked {
		t.Fatal("the checked row was not painted checked")
	}
}

func TestMenubarKeysMoveOpenAndChoose(t *testing.T) {
	m := &model{}
	m.bar.Open = fileID
	page, canvas := openPage(t, m)
	control := mustHit(t, canvas, "State")
	file := mustHit(t, canvas, "File")
	newRow := mustHit(t, canvas, "New window")

	result, ok := page.RouteKey(gfx.KeyEvent{Key: gfx.KeyDown, Target: control.ID})
	if !ok || !result.Handled {
		t.Fatalf("Down was not handled: %+v, %v", result, ok)
	}
	if m.bar.Open != fileID || result.Focus != newRow.ID {
		t.Fatalf("Down opened %v and focused %v, want File and the first row", m.bar.Open, result.Focus)
	}
	if m.keyTarget != control.ID {
		t.Fatalf("the key callback saw %v, want the control", m.keyTarget)
	}

	m.bar.Focus = fileID
	result, _ = page.RouteKey(gfx.KeyEvent{Key: gfx.KeyRight, Target: control.ID})
	if m.bar.Open != viewID {
		t.Fatalf("Right left %v open, want View", m.bar.Open)
	}
	view := mustHit(t, canvas, "View")
	if result.Focus != view.ID {
		t.Fatalf("Right focused %v, want the View trigger", result.Focus)
	}

	m.bar.Open = fileID
	m.bar.Focus = newID
	draw(page, canvas, m.view)
	newRow = mustHit(t, canvas, "New window")
	result, _ = page.RouteKey(gfx.KeyEvent{Key: gfx.KeyEnter, Target: control.ID})
	if result.Activate != newRow.ID || m.bar.Selected != newID {
		t.Fatalf("Enter activated %v and selected %v, want the New window row", result.Activate, m.bar.Selected)
	}

	result, _ = page.RouteKey(gfx.KeyEvent{Key: gfx.KeyEscape, Target: control.ID})
	if !m.bar.Open.IsZero() || result.Focus != file.ID {
		t.Fatalf("Escape left %v open and focused %v, want File", m.bar.Open, result.Focus)
	}
}

func TestMenubarPointerTranslation(t *testing.T) {
	m := &model{}
	m.bar.Open = fileID
	page, canvas := openPage(t, m)
	control := mustHit(t, canvas, "State")
	page.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Target: control.ID, X: control.Bounds.X + 4, Y: control.Bounds.Y + 4})
	if m.pointerTarget != control.ID {
		t.Fatalf("the pointer callback saw %v, want the control", m.pointerTarget)
	}
}

func TestMenubarDirectIdentitiesAreReported(t *testing.T) {
	m := &model{}
	page, canvas := openPage(t, m)
	row := mustHit(t, canvas, "File")
	drawClick(page, canvas, m.view, row.ID)
	if m.bar.Open != fileID {
		t.Fatalf("open = %v after a click on the internal trigger identity", m.bar.Open)
	}
}

func BenchmarkMenubarPaint(b *testing.B) {
	m := &model{grid: true}
	m.bar.Open = fileID
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.view, Theme: &theme})
	canvas := benchCanvas()
	view := m.view
	draw(page, canvas, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
