package sidebar_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/sidebar"
)

var (
	overviewID   = gfx.Item(1)
	componentsID = gfx.Item(2)
	benchmarksID = gfx.Item(3)
	buttonsID    = gfx.Item(4)
	inputsID     = gfx.Item(5)
	settingsID   = gfx.Item(6)
	accountID    = gfx.Item(7)
)

type model struct {
	sidebar  sidebar.State
	selected gfx.ID
	clicks   int
}

func (m *model) init() {
	m.sidebar.OnSelect = func(id gfx.ID) {
		m.clicks++
		m.selected = id
	}
}

func (m *model) declare(collapsed bool) {
	m.sidebar.Reset().Title("Viva Studio").Fixed().Collapsed(collapsed)
	if collapsed {
		m.sidebar.IconCollapse()
	}
	m.sidebar.Group("Workspace").
		Item(overviewID, "Overview").Icon("O").Badge("12").Active(true).
		Item(componentsID, "Components").Icon("C").
		Sub(buttonsID, "Buttons").SubActive(true).
		Sub(inputsID, "Inputs").
		Item(benchmarksID, "Benchmarks").Icon("B").
		Separator().
		Footer(settingsID, "Settings").Icon("S").
		Account(accountID, "Skyler").Icon("S")
}

func (m *model) view(c ui.C) {
	m.declare(false)
	sidebar.View(c, "Application navigation", &m.sidebar).Size(230, 420)
}

func (m *model) collapsedView(c ui.C) {
	m.declare(true)
	sidebar.View(c, "Application navigation", &m.sidebar).Size(56, 420)
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 320, Height: 460}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	canvas := new(gfx.Canvas)
	canvas.SetReduceMotion(true)
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	draw(page, canvas, view)
	return page, canvas
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func findHit(canvas *gfx.Canvas, id gfx.ID) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// The view paints the header, group, items, badge, sub items and footer, and
// the component's menu-button semantics carry the caller's identities.
func TestViewPaintsHeaderItemsBadgeAndFooter(t *testing.T) {
	m := new(model)
	m.init()
	_, canvas := open(t, m.view)

	for _, value := range []string{"Viva Studio", "Workspace", "Overview", "12", "Components", "Buttons", "Benchmarks", "Settings", "Skyler"} {
		if !hasText(canvas, value) {
			t.Fatalf("the sidebar did not paint %q", value)
		}
	}
	hit, ok := findHit(canvas, overviewID)
	if !ok {
		t.Fatal("the active item has no hit")
	}
	if hit.Role != gfx.RoleMenuItem || !hit.Selected {
		t.Fatalf("active item semantics = %+v, want a selected menu item", hit)
	}
	if _, ok := findHit(canvas, settingsID); !ok {
		t.Fatal("the footer item has no hit")
	}
}

// A click calls OnSelect with the caller's identity, and Clicked and Selected
// report it for the paint that delivered it.
func TestClickReportsTheCallerIdentity(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)

	drawClick(page, canvas, m.view, componentsID)
	if m.clicks != 1 || m.selected != componentsID {
		t.Fatalf("OnSelect ran %d times with %v, want once with %v", m.clicks, m.selected, componentsID)
	}
	if !m.sidebar.Clicked(componentsID) || m.sidebar.Selected() != componentsID {
		t.Fatal("Clicked or Selected did not report the clicked item")
	}

	draw(page, canvas, m.view)
	if m.clicks != 1 {
		t.Fatalf("an ordinary paint ran OnSelect %d times", m.clicks)
	}
	if m.sidebar.Clicked(componentsID) || !m.sidebar.Selected().IsZero() {
		t.Fatal("an ordinary paint reported a click")
	}
}

// A click on an identity the sidebar did not declare is not reported.
func TestForeignClickIsIgnored(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	drawClick(page, canvas, m.view, gfx.Item(99))
	if m.clicks != 0 || !m.sidebar.Selected().IsZero() {
		t.Fatalf("a foreign click was reported: %v / %v", m.clicks, m.sidebar.Selected())
	}
}

// Collapsed with IconCollapse keeps the icons and hides labels, groups and
// badges; the caller owns the collapsed state.
func TestIconCollapsedSidebarHidesLabelsAndKeepsIcons(t *testing.T) {
	m := new(model)
	m.init()
	_, canvas := open(t, m.collapsedView)
	if hasText(canvas, "Viva Studio") || hasText(canvas, "Overview") || hasText(canvas, "Workspace") {
		t.Fatal("the icon-collapsed sidebar painted a label or group")
	}
	if !hasText(canvas, "O") || !hasText(canvas, "C") {
		t.Fatal("the icon-collapsed sidebar did not paint its icons")
	}
	if _, ok := findHit(canvas, overviewID); !ok {
		t.Fatal("the icon-collapsed item has no hit")
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, m.view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.init()
	page, canvas := open(b, m.view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, m.view)
	}
}
