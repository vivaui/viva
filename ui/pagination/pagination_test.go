package pagination_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/pagination"
)

var changes int

func open(t *testing.T, state *pagination.State, spec pagination.Spec) *headless.Session {
	t.Helper()
	changes = 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			pagination.View(c, "Pages", state, spec)
			if state.Changed() {
				changes++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Pages", Width: 900, Height: 160}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// Next and Previous move one page, and the numbers jump; each change is reported
// once.
func TestControlsMoveThroughThePages(t *testing.T) {
	state := &pagination.State{PageSize: 10}
	session := open(t, state, pagination.Spec{Total: 95})
	if state.Page != 1 {
		t.Fatalf("Page = %d at the start, want 1", state.Page)
	}
	if err := session.ClickLabel("Next"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("5"); err != nil {
		t.Fatal(err)
	}
	if state.Page != 5 || changes != 2 {
		t.Fatalf("Page = %d after Next and 5, changes %d; want 5 and 2", state.Page, changes)
	}
	if err := session.ClickLabel("Previous"); err != nil {
		t.Fatal(err)
	}
	if state.Page != 4 {
		t.Fatalf("Page = %d after Previous, want 4", state.Page)
	}
}

// Previous is dimmed on the first page and Next on the last.
func TestEndsOfTheRangeCannotBePressed(t *testing.T) {
	state := &pagination.State{PageSize: 10, Page: 1}
	session := open(t, state, pagination.Spec{Total: 25})
	if err := session.ClickLabel("Previous"); err == nil {
		t.Fatal("Previous could be pressed on the first page")
	}
	state.Page = 3
	session.Frame()
	if err := session.ClickLabel("Next"); err == nil {
		t.Fatal("Next could be pressed on the last page")
	}
}

// A Total that shrinks brings the page back into range; the row of a long run of
// pages keeps the first and last page and a window around the current one.
func TestPageIsKeptInRangeAndLongRunsCollapse(t *testing.T) {
	state := &pagination.State{PageSize: 10, Page: 9}
	session := open(t, state, pagination.Spec{Total: 30})
	if state.Page != 3 {
		t.Fatalf("Page = %d after Total shrank to 3 pages, want 3", state.Page)
	}
	long := &pagination.State{PageSize: 10, Page: 10}
	session = open(t, long, pagination.Spec{Total: 200})
	buttons := map[string]bool{}
	for _, node := range session.Access() {
		if node.Role == gfx.RoleButton {
			buttons[node.Label] = true
		}
	}
	for _, shown := range []string{"1", "9", "10", "11", "20"} {
		if !buttons[shown] {
			t.Fatalf("page %s is not in the row", shown)
		}
	}
	for _, hidden := range []string{"2", "5", "15"} {
		if buttons[hidden] {
			t.Fatalf("page %s is in the row though it collapsed", hidden)
		}
	}
}

// Changing the page size keeps the first row of the page in view.
func TestPageSizeChangeKeepsTheFirstRow(t *testing.T) {
	state := &pagination.State{PageSize: 10, Page: 5} // rows 40..49
	session := open(t, state, pagination.Spec{Total: 200, SizeOptions: []int{10, 20, 50}})
	if err := session.ClickLabel("Rows per page"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("20"); err != nil {
		t.Fatal(err)
	}
	if state.PageSize != 20 || state.Page != 3 {
		t.Fatalf("PageSize %d, Page %d after choosing 20 on page 5 of 10, want 20 and 3", state.PageSize, state.Page)
	}
}

// The jumper takes a page number and goes to it, clamped.
func TestJumperGoesToThePage(t *testing.T) {
	state := &pagination.State{PageSize: 10}
	session := open(t, state, pagination.Spec{Total: 95, Jumper: true})
	if err := session.ClickLabel("Go to page"); err != nil {
		t.Fatal(err)
	}
	if err := session.Type("7"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if state.Page != 7 {
		t.Fatalf("Page = %d after typing 7 and Enter, want 7", state.Page)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	state := &pagination.State{PageSize: 10, Page: 5}
	spec := pagination.Spec{Total: 500, ShowTotal: true, SizeOptions: []int{10, 20, 50}, Jumper: true}
	view := func(c ui.C) { pagination.View(c, "Pages", state, spec) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 900, Height: 160}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	state := &pagination.State{PageSize: 10, Page: 5}
	spec := pagination.Spec{Total: 500, ShowTotal: true}
	view := func(c ui.C) { pagination.View(c, "Pages", state, spec) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 900, Height: 160}, view)
		canvas.EndFrame()
	}
}
