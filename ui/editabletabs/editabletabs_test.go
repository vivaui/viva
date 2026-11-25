package editabletabs_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/editabletabs"
)

type model struct {
	strip  *editabletabs.EditableTabs
	tabs   []editabletabs.Tab
	active int
	closed []int
	added  int
}

func newModel(titles ...string) *model {
	m := &model{strip: editabletabs.New()}
	for _, title := range titles {
		m.tabs = append(m.tabs, editabletabs.Tab{Title: title})
	}
	return m
}

func (m *model) open(t *testing.T, width int, addable bool) *headless.Session {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Children(func() {
			m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: addable}).FillWidth()
			// The page owns the tabs: it removes what the strip reports closed.
			if i, ok := m.strip.Closed(); ok {
				m.closed = append(m.closed, i)
				m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
				if m.active > i || m.active >= len(m.tabs) {
					m.active = max(m.active-1, 0)
				}
			}
			if m.strip.Added() {
				m.added++
				m.tabs = append(m.tabs, editabletabs.Tab{Title: "new"})
				m.active = len(m.tabs) - 1
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Tabs", Width: width, Height: 160}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// A click on a tab makes it active.
func TestClickActivates(t *testing.T) {
	m := newModel("alpha", "beta", "gamma")
	session := m.open(t, 600, false)
	if err := session.ClickLabel("gamma"); err != nil {
		t.Fatal(err)
	}
	if m.active != 2 {
		t.Fatalf("active = %d after a click on gamma, want 2", m.active)
	}
}

// The close button of a tab, the Delete key and a middle click each report the
// tab, and a pinned tab offers none of them.
func TestClosingReportsTheTab(t *testing.T) {
	m := newModel("alpha", "beta", "gamma")
	m.tabs[0].Pinned = true
	session := m.open(t, 600, false)
	closeButtons := 0
	var first gfx.Hit
	for _, hit := range session.Driver().Snapshot().Hits {
		if hit.Label == "Close" {
			if closeButtons == 0 {
				first = hit
			}
			closeButtons++
		}
	}
	if closeButtons != 2 {
		t.Fatalf("close buttons = %d, want one for each tab that is not pinned (2)", closeButtons)
	}
	if err := session.Click(first.Bounds.X+first.Bounds.Width/2, first.Bounds.Y+first.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if len(m.closed) != 1 || m.closed[0] != 1 {
		t.Fatalf("closed = %v after the close button of beta, want [1]", m.closed)
	}
	// Delete on the focused tab closes it; the pinned first tab is not closed by it.
	if err := session.ClickLabel("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("delete"); err != nil {
		t.Fatal(err)
	}
	if len(m.closed) != 1 {
		t.Fatalf("Delete closed a pinned tab: %v", m.closed)
	}
	if err := session.Keys("right"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("delete"); err != nil {
		t.Fatal(err)
	}
	if len(m.closed) != 2 || m.closed[1] != 1 {
		t.Fatalf("closed = %v after Right and Delete, want [1 1]", m.closed)
	}
}

func TestMiddleClickClosesATab(t *testing.T) {
	m := newModel("alpha", "beta")
	session := m.open(t, 600, false)
	hit, err := session.Locate("beta")
	if err != nil {
		t.Fatal(err)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/3, hit.Bounds.Y+hit.Bounds.Height/2
	if err := session.Driver().ClickButton(x, y, gfx.PointerButtonMiddle); err != nil {
		t.Fatal(err)
	}
	if len(m.closed) != 1 || m.closed[0] != 1 {
		t.Fatalf("closed = %v after a middle click on beta, want [1]", m.closed)
	}
}

// The arrow keys move the focus and make the tab active, Home and End go to the
// ends, and the add button is reached with Right and pressed with Enter.
func TestKeysMoveThroughTheStripAndAdd(t *testing.T) {
	m := newModel("alpha", "beta", "gamma")
	session := m.open(t, 600, true)
	if err := session.Keys("tab"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key  string
		want int
	}{{"right", 1}, {"end", 2}, {"left", 1}, {"home", 0}} {
		if err := session.Keys(step.key); err != nil {
			t.Fatal(err)
		}
		if m.active != step.want {
			t.Fatalf("active = %d after %q, want %d", m.active, step.key, step.want)
		}
	}
	session.Keys("end")
	session.Keys("right") // past the last tab: onto the add button
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if m.added != 1 || len(m.tabs) != 4 || m.active != 3 {
		t.Fatalf("added %d, tabs %d, active %d after Enter on the add button; want 1, 4, 3", m.added, len(m.tabs), m.active)
	}
}

// The add button is a button of its own, and absent unless asked for.
func TestAddButtonOnlyWhenAddable(t *testing.T) {
	for _, addable := range []bool{false, true} {
		m := newModel("alpha")
		session := m.open(t, 600, addable)
		_, err := session.Locate("Add tab")
		if (err == nil) != addable {
			t.Fatalf("addable %v: add button present = %v", addable, err == nil)
		}
	}
}

// A strip longer than its room scrolls with the wheel and keeps the active tab in view.
func TestLongStripScrollsAndKeepsTheActiveTabInView(t *testing.T) {
	m := newModel("first tab", "second tab", "third tab", "fourth tab", "fifth tab", "sixth tab")
	session := m.open(t, 300, false)
	if err := session.ClickLabel("first tab"); err != nil {
		t.Fatal(err)
	}
	session.Keys("end")
	hit, err := session.Locate("sixth tab")
	if err != nil {
		t.Fatal(err)
	}
	if hit.Bounds.X+hit.Bounds.Width > 300.5 || hit.Bounds.X < -0.5 {
		t.Fatalf("the active tab is at %+v, outside a 300 wide strip", hit.Bounds)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := newModel("alpha", "beta", "gamma")
	m.tabs[0].Pinned = true
	view := func(c ui.C) {
		m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 600, Height: 160}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := newModel("alpha", "beta", "gamma", "delta")
	view := func(c ui.C) {
		m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 600, Height: 160}, view)
		canvas.EndFrame()
	}
}
