package flex_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/ui"
)

var screenTheme = ui.LightTheme()

func drawScreen(screen *flex.Screen, canvas *ui.Canvas, body func(flex.Screen)) {
	canvas.Reset()
	canvas.BeginFrame()
	screen.Draw(canvas, ui.Rect{Width: 600, Height: 400}, body)
	canvas.EndFrame()
}

func newTestScreen() (*flex.Screen, *ui.Canvas) {
	canvas := &ui.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	return flex.NewScreen(flex.ScreenConfig{Theme: &screenTheme}), canvas
}

func captured(code string) bool {
	for _, event := range diag.Recent(64) {
		if event.Code == code {
			return true
		}
	}
	return false
}

func TestScreenReportsUnkeyedButtonsWithOneLabel(t *testing.T) {
	diag.Capture(64)
	defer diag.Capture(0)
	screen, canvas := newTestScreen()
	drawScreen(screen, canvas, func(s flex.Screen) {
		s.Column().Children(func() {
			for range 3 {
				s.Row().Children(func() { s.Button("Delete") })
			}
		})
	})
	if !captured("screen-duplicate-button") {
		t.Fatal("three unkeyed Delete buttons were not reported")
	}

	diag.Capture(64)
	keyed, canvas := newTestScreen()
	drawScreen(keyed, canvas, func(s flex.Screen) {
		for i := range 3 {
			s.KeyIndex(i).Button("Delete")
		}
	})
	if captured("screen-duplicate-button") {
		t.Fatal("keyed rows were reported")
	}
}

func TestScreenReportsAClickLostToACopy(t *testing.T) {
	diag.Capture(64)
	defer diag.Capture(0)
	model := map[string]bool{"a": false}
	screen, canvas := newTestScreen()
	body := func(s flex.Screen) {
		done := model["a"] // a copy: a new address in every paint
		s.Checkbox(&done, "A")
	}
	drawScreen(screen, canvas, body)
	var id ui.ID
	for _, hit := range canvas.Hits() {
		if hit.Label == "A" {
			id = hit.ID
		}
	}
	canvas.Reset()
	canvas.BeginEventPass(id)
	drawScreen(screen, canvas, body)
	canvas.EndEventPass()
	if !captured("screen-lost-click") {
		t.Fatal("the click lost to a copy was not reported")
	}
}

// A row keyed inside Key still binds a copy: its controls are numbered in the row.
func TestScreenCopyInAKeyRowKeepsItsClick(t *testing.T) {
	model := map[string]bool{"a": false}
	screen, canvas := newTestScreen()
	body := func(s flex.Screen) {
		r := s.Key("a")
		done := model["a"]
		if r.Checkbox(&done, "A").Changed() {
			model["a"] = done
		}
	}
	drawScreen(screen, canvas, body)
	var id ui.ID
	for _, hit := range canvas.Hits() {
		if hit.Label == "A" {
			id = hit.ID
		}
	}
	canvas.Reset()
	canvas.BeginEventPass(id)
	drawScreen(screen, canvas, body)
	canvas.EndEventPass()
	if !model["a"] {
		t.Fatal("the copy in a keyed row did not take its click")
	}
}

// Keys that come and go reuse numbers: the Tree grows with the rows on screen at
// once, not with every key the Screen has seen.
func TestScreenKeysKeepTheTreeBounded(t *testing.T) {
	screen, canvas := newTestScreen()
	var tree ui.Tree
	canvas.SetTree(&tree)
	screen = flex.NewScreen(flex.ScreenConfig{Tree: &tree, Theme: &screenTheme})
	page := 0
	body := func(s flex.Screen) {
		for i := range 10 {
			s.KeyIndex(page*10 + i).Button("Open")
		}
	}
	for page = range 50 {
		drawScreen(screen, canvas, body)
	}
	settled := tree.Len()
	for page = 50; page < 10_000; page++ {
		drawScreen(screen, canvas, body)
	}
	if tree.Len() != settled {
		t.Fatalf("the tree grew from %d to %d places over 100,000 keys", settled, tree.Len())
	}
	if settled > 100 {
		t.Fatalf("the tree has %d places for 10 rows on screen", settled)
	}
}

func TestScreenRetainedHeapStaysFlat(t *testing.T) {
	screen, canvas := newTestScreen()
	page := 0
	body := func(s flex.Screen) {
		for i := range 20 {
			s.KeyIndex(page*20 + i).Text("row")
		}
	}
	heap := func() uint64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return stats.HeapAlloc
	}
	for page = range 200 {
		drawScreen(screen, canvas, body)
	}
	before := heap()
	for page = 200; page < 5_200; page++ {
		drawScreen(screen, canvas, body)
	}
	if after := heap(); after > before+256<<10 {
		t.Fatalf("the heap grew from %d to %d bytes over 100,000 keys", before, after)
	}
}

type fleetHost struct {
	name, addr string
	handled    bool
}

type fleetModel struct {
	hosts   []fleetHost
	filter  int
	scans   int
	labels  []string
	summary string
}

func newFleetModel() *fleetModel {
	m := &fleetModel{labels: []string{"All", "Open", "Handled"}, summary: "8 hosts"}
	for i := range 8 {
		m.hosts = append(m.hosts, fleetHost{name: "host-" + string(rune('a'+i)), addr: "10.0.0.1", handled: i%2 == 0})
	}
	return m
}

func (m *fleetModel) page(s flex.Screen) {
	s.Column().Fill().Pad(22).Gap(14).Children(func() {
		s.Text("Fleet check").FontSize(26).Bold()
		s.Choice(&m.filter, m.labels)
		s.ScrollArea("list").Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				h := &m.hosts[i]
				s.Row().FillWidth().Pad(12).Gap(12).Children(func() {
					s.Text(h.name).Bold()
					s.Text(h.addr).Muted()
					s.Checkbox(&h.handled, "")
				})
			}
		})
		s.Row().Gap(10).Children(func() {
			s.Text(m.summary).Muted()
			if s.Button("Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

func TestScreenSteadyPaintAllocatesNothing(t *testing.T) {
	screen, canvas := newTestScreen()
	m := newFleetModel()
	for range 3 {
		drawScreen(screen, canvas, m.page)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawScreen(screen, canvas, m.page) }); allocs != 0 {
		t.Fatalf("a steady Screen paint allocates %v times", allocs)
	}
}

func TestScreenSteadyKeyedPaintAllocatesNothing(t *testing.T) {
	screen, canvas := newTestScreen()
	names := []string{"alpha", "beta", "gamma", "delta"}
	body := func(s flex.Screen) {
		for _, name := range names {
			r := s.Key(name)
			r.Row().Children(func() {
				r.Text(name)
				r.Button("Open")
			})
		}
	}
	for range 3 {
		drawScreen(screen, canvas, body)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawScreen(screen, canvas, body) }); allocs != 0 {
		t.Fatalf("a steady keyed paint allocates %v times", allocs)
	}
}

// A native view is declared where its element is painted: inside a Modal it is on the
// dialog's layer, inside a ScrollArea it is cut to the area, and the page's popups go
// over it.
func TestScreenNativeViewTakesItsPlaceInThePage(t *testing.T) {
	screen, canvas := newTestScreen()
	open := true
	var menu bool = true
	drawScreen(screen, canvas, func(s flex.Screen) {
		s.Column().Fill().Children(func() {
			anchor := s.Button("Menu")
			s.ScrollArea("pages").Height(100).Children(func() {
				s.Slot().Height(300).FillWidth().NativeView(1)
			})
			s.Popover(anchor, &menu, func(p flex.Screen) { p.Text("over the page") })
			s.Modal(&open, "Preview", func(m flex.ModalScreen) {
				m.Slot().Size(100, 80).NativeView(2)
			})
		})
	})
	var page, preview ui.NativeView
	for _, view := range canvas.NativeViews() {
		switch view.ID {
		case 1:
			page = view
		case 2:
			preview = view
		}
	}
	if page.ID == 0 || preview.ID == 0 {
		t.Fatalf("native views = %+v, want both", canvas.NativeViews())
	}
	if !page.HasClip || page.ClipOf.Height > 100.5 {
		t.Errorf("the page in a scroll area is clipped to %+v (clipped %v), want the area", page.ClipOf, page.HasClip)
	}
	if preview.Layer <= page.Layer {
		t.Errorf("the view in the dialog is on layer %d, the page's on %d: want it on the dialog's", preview.Layer, page.Layer)
	}
	var popoverLayer uint16
	for _, text := range canvas.Texts() {
		if text.Value == "over the page" {
			popoverLayer = text.Layer
		}
	}
	if popoverLayer <= page.Layer {
		t.Errorf("the popover is on layer %d, not over the page's view on %d", popoverLayer, page.Layer)
	}
}

func TestScreenTabsAndSelectWriteTheirIndex(t *testing.T) {
	screen, canvas := newTestScreen()
	tab, size := 0, -1
	var state flex.SelectState
	labels := []string{"S", "M", "L"}
	body := func(s flex.Screen) {
		s.Column().Children(func() {
			s.Tabs(&tab, []string{"General", "Keys"})
			s.Select(&state, &size, labels)
		})
	}
	drawScreen(screen, canvas, body)
	click := func(label string) {
		t.Helper()
		var id ui.ID
		for _, hit := range canvas.Hits() {
			if hit.Label == label && !hit.IsLabel {
				id = hit.ID
			}
		}
		if id.IsZero() {
			t.Fatalf("no hit %q in %d hits", label, len(canvas.Hits()))
		}
		canvas.Reset()
		canvas.BeginEventPass(id)
		drawScreen(screen, canvas, body)
		canvas.EndEventPass()
		drawScreen(screen, canvas, body)
	}
	click("Keys")
	if tab != 1 {
		t.Fatalf("tab = %d, want 1", tab)
	}
	var trigger string
	for _, hit := range canvas.Hits() {
		if hit.Role == ui.RoleCombobox {
			trigger = hit.Label
		}
	}
	click(trigger)
	if !state.Open {
		t.Fatal("a click on the trigger did not open the list")
	}
	click("L")
	if size != 2 || state.Open {
		t.Fatalf("size = %d open = %v, want 2 and closed", size, state.Open)
	}
}

func TestScreenDeclaringAfterEndPanics(t *testing.T) {
	screen, canvas := newTestScreen()
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "after the page was laid out") {
			t.Fatalf("declaring in a Draw callback: recover() = %v", r)
		}
	}()
	drawScreen(screen, canvas, func(s flex.Screen) {
		s.Slot().Size(10, 10).Draw(func(*flex.DrawContext) { s.Text("late") })
	})
}
