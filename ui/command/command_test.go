package command_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/command"
	"github.com/vivaui/viva/window"
)

func testTheme() *gfx.Theme {
	theme := gfx.LightTheme()
	return &theme
}

func drawPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 640, Height: 240}, view)
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

type model struct {
	command     command.Command
	page        *ui.Page
	canvas      *gfx.Canvas
	selected    int
	selectedSet bool
	chosen      int
	chosenSet   bool
	filter      []string
}

func (m *model) wire() {
	m.command.Query = "open"
	m.command.Results = []command.Result{
		{Kind: command.ResultGroup, Label: "Suggestions"},
		{ID: gfx.Item(11), Label: "Open workspace", Shortcut: "⌘O"},
		{ID: gfx.Item(12), Label: "Create window", Shortcut: "⌘N"},
		{Kind: command.ResultSeparator},
		{ID: gfx.Item(13), Label: "Pin current view", Disabled: true},
	}
	m.command.Filter = func(query string) { m.filter = append(m.filter, query) }
	m.command.Select = func(index int) {
		m.selected, m.selectedSet = index, true
	}
	m.command.Choose = func(index int) {
		m.chosen, m.chosenSet = index, true
	}
}

func (m *model) viewPage(c ui.C) {
	command.View(c, "Command", &m.command).Size(480, 210)
}

func (m *model) open(t testing.TB) {
	t.Helper()
	theme := testTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: theme})
	m.canvas = new(gfx.Canvas)
	drawPage(m.page, m.canvas, m.viewPage)
}

func control(t *testing.T, canvas *gfx.Canvas) gfx.Hit {
	t.Helper()
	hit, ok := hitLabelled(canvas, "Command")
	if !ok {
		t.Fatal("the control painted no hit")
	}
	return hit
}

func TestViewPaintsResultsAndGeometry(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	ctrl := control(t, m.canvas)
	if ctrl.Role != gfx.RoleCombobox || !ctrl.Focusable || ctrl.Value != "open" {
		t.Fatalf("control hit = %+v, want the focusable query row of the palette", ctrl)
	}
	item := func(label string) gfx.Hit {
		t.Helper()
		hit, ok := hitLabelled(m.canvas, label)
		if !ok {
			t.Fatalf("no hit labelled %q", label)
		}
		return hit
	}
	first := item("Open workspace")
	if first.ID != gfx.Item(11) || !first.Selected || first.Role != gfx.RoleOption {
		t.Fatalf("first item hit = %+v, want the caller's selected option", first)
	}
	if second := item("Create window"); second.ID != gfx.Item(12) || second.Selected {
		t.Fatalf("second item hit = %+v, want the caller's unselected option", second)
	}
	pinned := item("Pin current view")
	if !pinned.Disabled || pinned.Focusable {
		t.Fatalf("disabled item hit = %+v, want inert", pinned)
	}

	if bounds := m.command.Bounds(); bounds.Width <= 0 || bounds.Height <= 0 {
		t.Fatalf("bounds = %v, want the painted surface", bounds)
	}
	input := m.command.InputBounds()
	if input.Height != 36 || input.Width <= 0 {
		t.Fatalf("input bounds = %v, want the compact 36-pixel row", input)
	}
	if row := m.command.ItemBounds(0); row.Height != 32 || row.Y <= input.Y+input.Height-32 {
		t.Fatalf("item bounds = %v, want a 32-pixel row under the input", row)
	}
	if m.command.ItemBounds(3) != (gfx.Rect{}) || m.command.ItemBounds(-1) != (gfx.Rect{}) {
		t.Fatal("ItemBounds reported a row that is not an item")
	}
}

// Dialog is a chained option, not a second constructor, and changes the row
// geometry the component paints.
func TestDialogOptionChangesGeometry(t *testing.T) {
	m := new(model)
	m.wire()
	m.command.Options = m.command.Options.Dialog()
	m.open(t)

	if input := m.command.InputBounds(); input.Height != 48 {
		t.Fatalf("dialog input bounds = %v, want a 48-pixel row", input)
	}
	if row := m.command.ItemBounds(0); row.Height != 44 {
		t.Fatalf("dialog item bounds = %v, want a 44-pixel row", row)
	}
}

func TestNativeEditingUpdatesQueryAndFilters(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	input := m.page.TextInput(window.TextInputContext{ID: control(t, m.canvas).ID})
	if input == nil || input.Editor == nil || !input.NavigationKeys {
		t.Fatalf("text input = %+v, want the wrapper's editor with navigation keys", input)
	}
	input.Editor.Insert("x")
	drawPage(m.page, m.canvas, m.viewPage)

	if m.command.Query != "openx" {
		t.Fatalf("query = %q, want the typed value", m.command.Query)
	}
	if len(m.filter) != 1 || m.filter[0] != "openx" {
		t.Fatalf("filter calls = %v, want the typed value once", m.filter)
	}

	// An assignment between paints replaces the query without filtering: the
	// caller already owns the results it is about to paint.
	m.command.Query = "create"
	drawPage(m.page, m.canvas, m.viewPage)
	if input := m.page.TextInput(window.TextInputContext{ID: control(t, m.canvas).ID}); input.Editor.String() != "create" {
		t.Fatalf("editor = %q, want the assigned query", input.Editor.String())
	}
	if len(m.filter) != 1 {
		t.Fatalf("filter calls = %v, want no call for an assignment", m.filter)
	}
}

func TestKeysMoveSelectionAndChoose(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)
	id := control(t, m.canvas).ID

	down := func() gfx.EventResult {
		return m.page.Key(gfx.KeyEvent{Key: gfx.KeyDown, Target: id})
	}
	if result := down(); !result.Handled || !result.Repaint || m.command.Selected != 1 || !m.selectedSet || m.selected != 1 {
		t.Fatalf("first down: selected=%d callback=%d/%t result=%+v", m.command.Selected, m.selected, m.selectedSet, result)
	}
	// Item 2 is disabled, so the next step wraps to the first item.
	if result := down(); !result.Handled || m.command.Selected != 0 || m.selected != 0 {
		t.Fatalf("wrapping down: selected=%d callback=%d result=%+v", m.command.Selected, m.selected, result)
	}
	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyUp, Target: id}); !result.Handled || m.command.Selected != 1 {
		t.Fatalf("up: selected=%d result=%+v", m.command.Selected, result)
	}
	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyEnter, Target: id}); !result.Handled || !m.chosenSet || m.chosen != 1 {
		t.Fatalf("enter: chosen=%d/%t result=%+v", m.chosen, m.chosenSet, result)
	}

	// Enter on a disabled row does nothing and stays with the page.
	m.command.Selected = 2
	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyEnter, Target: id}); result.Handled {
		t.Fatalf("enter on a disabled item was handled: %+v", result)
	}
}

func TestPointerMovesAndClicksSelectEnabledItems(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	m.canvas.SetInteraction(gfx.Item(12), gfx.ID{}, gfx.ID{}, false)
	drawPage(m.page, m.canvas, m.viewPage)
	if m.command.Selected != 1 || m.selected != 1 {
		t.Fatalf("hover selected %d (callback %d), want item 1", m.command.Selected, m.selected)
	}

	// A hover over a disabled row leaves the selection alone.
	m.canvas.SetInteraction(gfx.Item(13), gfx.ID{}, gfx.ID{}, false)
	drawPage(m.page, m.canvas, m.viewPage)
	if m.command.Selected != 1 {
		t.Fatalf("hover on a disabled row selected %d, want item 1", m.command.Selected)
	}

	m.canvas.BeginEventPass(gfx.Item(11), gfx.ClickInfo{})
	drawPage(m.page, m.canvas, m.viewPage)
	m.canvas.EndEventPass()
	if m.command.Selected != 0 {
		t.Fatalf("click selected %d, want item 0", m.command.Selected)
	}
}

// A warm repaint allocates nothing: the wrapper keeps its handlers, draw
// function and row geometry, and the component keeps its own caches.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)
	if allocations := testing.AllocsPerRun(100, func() { drawPage(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewWarmPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.open(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		drawPage(m.page, m.canvas, m.viewPage)
	}
}
