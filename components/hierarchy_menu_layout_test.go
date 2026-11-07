package components

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func deepHierarchyMenu() (HierarchyMenuState, HierarchyMenuOptions) {
	var tree ui.Tree
	f := tree.Feature("deep-hierarchy-menu")
	o := HierarchyMenuOptions{
		OptionIDs: tree.Sub(f, tree.Name("option")), ScrollID: tree.ID(f, "scroll"),
		Scope: tree.ID(f, "scope"), Layer: 4, Label: "Groups",
		Viewport: ui.Rect{Width: 716, Height: 600}, Overlay: true, DismissID: tree.ID(f, "dismiss"),
	}
	values := []string{"A/B/C/D/E", "A/B/C/D/F"}
	var m HierarchyMenuState
	m.Build("/", values[0], nil, len(values), func(i int) HierarchyMenuItem { return HierarchyMenuItem{Value: values[i]} })
	return m, o
}

func TestHierarchyMenuKeepsVisibleLevelsReadableInsideViewport(t *testing.T) {
	for _, levels := range []int{1, 2, 5, hierarchyMaxLevels} {
		for _, width := range []float32{320, 716, 1400} {
			t.Run(fmt.Sprintf("levels=%d/width=%.0f", levels, width), func(t *testing.T) {
				m, o := deepHierarchyMenu()
				o.Viewport = ui.Rect{X: 70, Y: 30, Width: width, Height: 600}
				value := strings.Repeat("Wide group with many words/", levels-1) + "Leaf"
				m.Build("/", value, nil, 1, func(int) HierarchyMenuItem { return HierarchyMenuItem{Value: value} })
				var canvas ui.Canvas
				anchor := ui.Rect{X: width + 20, Y: 60, Width: 160, Height: 26}
				PaintHierarchyMenu(&canvas, ui.LightTheme(), anchor, &m, o)
				first := 0
				if m.compact {
					first = m.depth
				}
				for level := first; level <= m.depth; level++ {
					panel := m.panels[level]
					if panel.Width < hierarchyMinColumn || panel.X < o.Viewport.X || panel.X+panel.Width > o.Viewport.X+o.Viewport.Width {
						t.Fatalf("level %d panel %+v is not usable inside %+v", level, panel, o.Viewport)
					}
					for ancestor := first; ancestor < level; ancestor++ {
						previous := m.panels[ancestor]
						if min(panel.X+panel.Width, previous.X+previous.Width)-max(panel.X, previous.X) > 4.01 {
							t.Fatalf("level %d panel %+v covers ancestor %+v", level, panel, previous)
						}
					}
				}
				root := m.panels[first]
				if root.X >= anchor.X+anchor.Width || root.X+root.Width <= anchor.X {
					t.Fatalf("root popup %+v detached from anchor %+v", root, anchor)
				}
				visible := 0
				for _, hit := range canvas.Hits() {
					if _, ok := hit.ID.In(o.OptionIDs); ok && hit.Role == ui.RoleOption {
						visible++
						if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
							t.Fatalf("empty row hit: %+v", hit)
						}
					}
				}
				if visible != levels-first {
					t.Fatalf("%d visible rows, want %d", visible, levels-first)
				}
				for _, text := range canvas.Texts() {
					if text.Value == "" || text.Value == "…" || text.Bounds.Width <= 0 {
						t.Fatalf("menu lost its readable label: %+v", text)
					}
				}
			})
		}
	}
}

func TestHierarchyMenuPointerNavigatesDeepCascade(t *testing.T) {
	for _, anchorX := range []float32{10, 500, 1200} {
		t.Run(fmt.Sprintf("anchor=%.0f", anchorX), func(t *testing.T) {
			m, o := deepHierarchyMenu()
			o.Viewport = ui.Rect{Width: 1400, Height: 600}
			values := []string{"A/B/C/D/E/F/G/H"}
			for level := 0; level < hierarchyMaxLevels; level++ {
				values = append(values, strings.Join(strings.Split(values[0], "/")[:level], "/")+"/Other")
				values[len(values)-1] = strings.TrimPrefix(values[len(values)-1], "/")
			}
			m.Build("/", "", nil, len(values), func(i int) HierarchyMenuItem { return HierarchyMenuItem{Value: values[i]} })
			chosen := ""
			driver, err := app.NewDriver(app.Config{Width: 1400, Height: 600, Paint: func(c *ui.Canvas, _ ui.Size) {
				if index, ok := c.ClickedIn(o.OptionIDs); ok {
					chosen, _ = m.Choose(index)
				}
				PaintHierarchyMenu(c, ui.LightTheme(), ui.Rect{X: anchorX, Y: 20, Width: 160, Height: 26}, &m, o)
			}, OnKey: func(event ui.KeyEvent) ui.EventResult { return m.Key(event, true).EventResult }}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			row := func(level, index int) ui.Rect {
				t.Helper()
				id := o.OptionIDs.At(level<<16 | index)
				for _, hit := range driver.Snapshot().Hits {
					if hit.ID == id {
						x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
						if target, ok := driver.HitTest(x, y); !ok || target != id {
							t.Fatalf("level %d row %d is covered", level, index)
						}
						return hit.Bounds
					}
				}
				t.Fatalf("missing level %d row %d", level, index)
				return ui.Rect{}
			}
			move := func(level, index int) {
				t.Helper()
				bounds := row(level, index)
				must(driver.Move(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2))
				must(driver.Advance(hierarchySwitchDelay + time.Millisecond))
			}
			for level := 0; level < hierarchyMaxLevels-1; level++ {
				move(level, 0)
				if m.depth != level+1 {
					t.Fatalf("hovering level %d opened depth %d", level, m.depth)
				}
				must(driver.Advance(time.Second))
				if m.depth != level+1 {
					t.Fatalf("resting pointer changed depth to %d", m.depth)
				}
			}
			for level := hierarchyMaxLevels - 1; level >= 0; level-- {
				move(level, 1)
				if m.depth != level {
					t.Fatalf("returning to level %d left depth %d", level, m.depth)
				}
				bounds := row(level, 1)
				must(driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2))
				want := values[level+1]
				if chosen != want {
					t.Fatalf("ancestor click chose %q, want %q", chosen, want)
				}
			}
			// Keyboard navigation still walks in and out of the same levels.
			move(0, 0)
			for level := 1; level < hierarchyMaxLevels; level++ {
				must(driver.Key(ui.KeyRight, 0))
				if m.level != level {
					t.Fatalf("Right reached level %d, want %d", m.level, level)
				}
			}
			for level := hierarchyMaxLevels - 2; level >= 0; level-- {
				must(driver.Key(ui.KeyLeft, 0))
				if m.depth != level {
					t.Fatalf("Left left depth %d, want %d", m.depth, level)
				}
			}
		})
	}
}

func hierarchyTestHit(t *testing.T, driver *app.Driver, id ui.ID) ui.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return hit.Bounds
		}
	}
	t.Fatalf("missing hierarchy hit %v", id)
	return ui.Rect{}
}

func hierarchyTestClick(t *testing.T, driver *app.Driver, id ui.ID) {
	t.Helper()
	bounds := hierarchyTestHit(t, driver, id)
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
}

// Click itself moves the pointer before pressing. Opening a fly-out on that
// move must not replace the parent row with its child before the press arrives.
func TestHierarchyMenuDirectClickStillChoosesItsParent(t *testing.T) {
	for _, path := range []string{"A/B", "A/B/C/D/E/F/G/H"} {
		t.Run(path, func(t *testing.T) {
			m, o := deepHierarchyMenu()
			values := []string{path, "Other"}
			m.Build("/", "", nil, len(values), func(i int) HierarchyMenuItem { return HierarchyMenuItem{Value: values[i]} })
			chosen := ""
			driver, err := app.NewDriver(app.Config{Width: 716, Height: 600, Paint: func(c *ui.Canvas, _ ui.Size) {
				if index, ok := c.ClickedIn(o.OptionIDs); ok {
					chosen, _ = m.Choose(index)
				}
				PaintHierarchyMenu(c, ui.LightTheme(), ui.Rect{X: 500, Y: 20, Width: 160, Height: 26}, &m, o)
			}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			before := hierarchyTestHit(t, driver, o.OptionIDs.At(0))
			hierarchyTestClick(t, driver, o.OptionIDs.At(0))
			if chosen != "A" {
				t.Fatalf("direct parent click chose %q, want A", chosen)
			}
			if after := hierarchyTestHit(t, driver, o.OptionIDs.At(0)); after != before {
				t.Fatalf("opening a child moved its parent: %+v -> %+v", before, after)
			}
		})
	}
}

// SingleSelect forwards the hierarchy's navigation hits through Choose. They
// must repaint the next level without changing the value or closing the menu.
func TestSingleSelectCompactHierarchyNavigatesAndSelects(t *testing.T) {
	s, o := hierarchicalSelectFixture()
	const path = "A/B/C/D/E/F/G/H"
	s.SetOptions([]SingleSelectOption{{Value: path}, {Value: "Other"}})
	s.Open = true
	driver, err := app.NewDriver(app.Config{Width: 320, Height: 600, Paint: func(c *ui.Canvas, size ui.Size) {
		o.Viewport = ui.Rect{Width: size.Width, Height: size.Height}
		if index, ok := c.ClickedIn(o.OptionIDs); ok {
			s.Choose(index)
		}
		PaintSingleSelect(c, ui.LightTheme(), ui.Rect{X: 100, Y: 20, Width: 160, Height: 32}, s, o)
	}, OnKey: s.Key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if !s.tree.compact {
		t.Fatal("eight levels in a narrow window need compact navigation")
	}
	for level := 0; level < hierarchyMaxLevels-1; level++ {
		bounds := hierarchyTestHit(t, driver, o.OptionIDs.At(level<<16))
		if err := driver.Move(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
			t.Fatal(err)
		}
		if err := driver.Advance(time.Second); err != nil {
			t.Fatal(err)
		}
		if s.tree.depth != level {
			t.Fatal("compact hover navigated before a click")
		}
		hierarchyTestClick(t, driver, o.OptionIDs.At((hierarchyOpenLevel+level)<<16))
		if !s.Open || s.Value() != "" || s.tree.depth != level+1 {
			t.Fatalf("navigation selected %q, open=%v depth=%d", s.Value(), s.Open, s.tree.depth)
		}
		label := strings.Split(path, "/")[level+1]
		visible := false
		for _, text := range driver.Snapshot().Texts {
			visible = visible || text.Value == label && text.Bounds.Width > 0
		}
		if !visible {
			t.Fatalf("current level %q has no readable text", label)
		}
	}
	// Resizing across the cascade threshold keeps keyboard navigation on a
	// displayed level, including after returning to the compact presentation.
	for _, width := range []int{1400, 320} {
		if err := driver.Resize(width, 600); err != nil {
			t.Fatal(err)
		}
		if s.tree.level != 7 {
			t.Fatalf("resize moved keyboard to level %d", s.tree.level)
		}
		hierarchyTestHit(t, driver, o.OptionIDs.At(7<<16))
	}
	for level := hierarchyMaxLevels - 2; level >= 0; level-- {
		hierarchyTestClick(t, driver, o.OptionIDs.At(hierarchyBackIndex))
		if !s.Open || s.Value() != "" || s.tree.depth != level {
			t.Fatalf("back selected %q, open=%v depth=%d", s.Value(), s.Open, s.tree.depth)
		}
	}
	// The parent label still selects that parent, rather than descending.
	hierarchyTestClick(t, driver, o.OptionIDs.At(0))
	if s.Open || s.Value() != "A" {
		t.Fatalf("parent label chose %q (open=%v)", s.Value(), s.Open)
	}
	if err := driver.Key(ui.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	for level := 1; level < hierarchyMaxLevels; level++ {
		if err := driver.Key(ui.KeyRight, 0); err != nil {
			t.Fatal(err)
		}
		if s.tree.level != level || s.tree.depth != level {
			t.Fatalf("Right reached level %d depth %d, want %d", s.tree.level, s.tree.depth, level)
		}
	}
	if err := driver.Key(ui.KeyLeft, 0); err != nil {
		t.Fatal(err)
	}
	if s.tree.depth != 6 {
		t.Fatalf("Left left depth %d", s.tree.depth)
	}
	if err := driver.Key(ui.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if s.Open || s.Value() != path {
		t.Fatalf("Enter chose %q (open=%v)", s.Value(), s.Open)
	}
}

func TestHierarchyMenuCompactBackRemainsReachableAfterScroll(t *testing.T) {
	m, o := deepHierarchyMenu()
	o.Viewport.Width, o.MaxRows = 320, 3
	values := []string{"A/B/C/D/E/F/G/H"}
	for i := 0; i < 30; i++ {
		values = append(values, fmt.Sprintf("A/Child %d", i))
	}
	m.Build("/", "A/B", nil, len(values), func(i int) HierarchyMenuItem { return HierarchyMenuItem{Value: values[i]} })
	driver, err := app.NewDriver(app.Config{Width: 320, Height: 600, Paint: func(c *ui.Canvas, _ ui.Size) {
		if index, ok := c.ClickedIn(o.OptionIDs); ok {
			m.Choose(index)
		}
		PaintHierarchyMenu(c, ui.LightTheme(), ui.Rect{X: 100, Y: 20, Width: 160, Height: 26}, &m, o)
	}, OnScroll: func(event ui.ScrollEvent) ui.EventResult {
		changed := m.Wheel(event)
		return ui.EventResult{Handled: changed, Repaint: changed}
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	back := hierarchyTestHit(t, driver, o.OptionIDs.At(hierarchyBackIndex))
	row := hierarchyTestHit(t, driver, o.OptionIDs.At(1<<16))
	if err := driver.Scroll(ui.ScrollEvent{X: row.X + 20, Y: row.Y + 16, DeltaY: 1000}); err != nil {
		t.Fatal(err)
	}
	if m.scrolls[1].Offset == 0 {
		t.Fatal("the child list did not scroll")
	}
	if after := hierarchyTestHit(t, driver, o.OptionIDs.At(hierarchyBackIndex)); after != back {
		t.Fatalf("scroll moved back button %+v -> %+v", back, after)
	}
	hierarchyTestClick(t, driver, o.OptionIDs.At(hierarchyBackIndex))
	if m.depth != 0 {
		t.Fatalf("back left depth %d", m.depth)
	}
}

func TestHierarchyMenuRebuildAndSeekKeepCurrentLevelReachable(t *testing.T) {
	m, o := deepHierarchyMenu()
	o.Viewport.Width = 320
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		PaintHierarchyMenu(&canvas, ui.LightTheme(), ui.Rect{X: 100, Y: 20, Width: 160, Height: 26}, &m, o)
	}
	paint()
	if !m.compact || m.level != m.depth {
		t.Fatal("deep selection did not show its compact level")
	}
	m.Reset()
	paint()
	if m.depth != 0 || m.level != 0 || m.panels[0].Width == 0 {
		t.Fatal("Reset did not return to a visible root")
	}
	m.Seek("A/B/C")
	paint()
	if m.depth != 2 || m.level != 2 || m.panels[2].Width == 0 {
		t.Fatal("Seek did not reveal the selected level")
	}
	m.Build("/", "New root", nil, 1, func(int) HierarchyMenuItem { return HierarchyMenuItem{Value: "New root"} })
	paint()
	if m.compact || m.depth != 0 || m.level != 0 {
		t.Fatal("rebuilding a shallow tree retained the deep layout")
	}
	if value, ok := m.Choose(0); !ok || value != "New root" {
		t.Fatalf("rebuilt menu chose %q (%v)", value, ok)
	}
}

func TestHierarchyMenuCompactNavigationClipsToShortPopup(t *testing.T) {
	for _, height := range []float32{40, 80, 120} {
		m, o := deepHierarchyMenu()
		o.Viewport.Width, o.Viewport.Height = 320, height
		var canvas ui.Canvas
		PaintHierarchyMenu(&canvas, ui.LightTheme(), ui.Rect{X: 100, Y: 20, Width: 160, Height: 26}, &m, o)
		panel := m.panels[m.depth]
		for _, hit := range canvas.Hits() {
			if _, ok := hit.ID.In(o.OptionIDs); ok && (hit.Bounds.Y < panel.Y || hit.Bounds.Y+hit.Bounds.Height > panel.Y+panel.Height) {
				t.Fatalf("height %.0f navigation %+v escaped its short popup %+v", height, hit.Bounds, panel)
			}
		}
	}
}

// A selected deep path opens several panels at once. All of their rows must
// remain clickable when the complete cascade fits in the window.
func TestHierarchyMenuDeepPathDoesNotCoverAncestorRows(t *testing.T) {
	m, o := deepHierarchyMenu()
	var canvas ui.Canvas
	PaintHierarchyMenu(&canvas, ui.LightTheme(), ui.Rect{X: 240, Y: 20, Width: 100, Height: 26}, &m, o)
	if m.compact {
		t.Fatal("a readable cascade fits beside the anchor")
	}
	for a := 0; a <= m.depth; a++ {
		for b := a + 1; b <= m.depth; b++ {
			first, second := m.panels[a], m.panels[b]
			width := min(first.X+first.Width, second.X+second.Width) - max(first.X, second.X)
			height := min(first.Y+first.Height, second.Y+second.Height) - max(first.Y, second.Y)
			// Adjacent popup borders overlap by four pixels; row contents must not.
			if width > 4 && height > 4 {
				t.Fatalf("level %d panel %+v covers level %d panel %+v by %.0fx%.0f", b, second, a, first, width, height)
			}
		}
	}
}

func BenchmarkHierarchyMenuDeepPathPaint(b *testing.B) {
	m, o := deepHierarchyMenu()
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		PaintHierarchyMenu(&canvas, ui.LightTheme(), ui.Rect{X: 240, Y: 20, Width: 100, Height: 26}, &m, o)
	}
	paint()
	b.ReportAllocs()
	for b.Loop() {
		paint()
	}
}

// Planning reserves every branch before the first hover; measure that cold
// work as well as the steady paint above on a representative larger tree.
func BenchmarkHierarchyMenuBuildAndFirstPaint(b *testing.B) {
	m, o := deepHierarchyMenu()
	o.Viewport.Width = 1400
	values := make([]HierarchyMenuItem, 200)
	for i := range values {
		values[i].Value = fmt.Sprintf("Group %02d/Branch %02d/Leaf %03d", i%20, i/20%5, i)
	}
	var canvas ui.Canvas
	paint := func() {
		m.Build("/", "", nil, len(values), func(i int) HierarchyMenuItem { return values[i] })
		canvas.Reset()
		PaintHierarchyMenu(&canvas, ui.LightTheme(), ui.Rect{X: 20, Y: 20, Width: 240, Height: 26}, &m, o)
	}
	paint()
	b.ReportAllocs()
	for b.Loop() {
		paint()
	}
}
