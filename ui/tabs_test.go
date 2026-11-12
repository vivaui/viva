package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	tabsScope = tree.Sub(page, tree.Name("tabs"))
	tabLabels = []string{"General", "Keys", "Advanced"}
)

// Tabs look exactly like the components.Tabs they are painted by, in each variant.
func TestTabsArePaintedLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		name     string
		declare  func(Box) Box
		direct   func(components.Tabs) components.Tabs
		bounds   gfx.Rect
		width    float32
		height   float32
		hovered  int
		disabled bool
	}{
		{"default", func(b Box) Box { return b }, func(t components.Tabs) components.Tabs { return t }, gfx.Rect{Width: 300, Height: 36}, 300, 0, 1, false},
		{"line", func(b Box) Box { return b.Line() }, func(t components.Tabs) components.Tabs { return t.Line() }, gfx.Rect{Width: 300, Height: 36}, 300, 0, -1, false},
		{"vertical", func(b Box) Box { return b.Vertical().Height(108) }, func(t components.Tabs) components.Tabs { return t.Vertical() }, gfx.Rect{Width: 160, Height: 108}, 160, 108, 2, false},
		{"disabled", func(b Box) Box { return b.Disabled(true) }, func(t components.Tabs) components.Tabs { return t.Disabled() }, gfx.Rect{Width: 300, Height: 36}, 300, 0, 1, true},
	} {
		flexed, direct := testCanvas(), testCanvas()
		if test.hovered >= 0 {
			flexed.SetInteraction(tabsScope.At(test.hovered), gfx.ID{}, gfx.ID{}, false)
			direct.SetInteraction(tabsScope.At(test.hovered), gfx.ID{}, gfx.ID{}, false)
		}
		var view View
		view.Draw(flexed, theme, gfx.Rect{Width: test.width, Height: 200}, func(f *Frame) {
			f.Column().Children(func() { test.declare(f.Tabs(tabsScope, tabLabels, 1)) })
		})
		test.direct(components.BeginTabList(direct, theme, test.bounds, tabsScope, tabLabels, 1)).Paint()
		sameCanvas(t, test.name, flexed, direct)
	}
}

func TestTabsReportTheClickedTabAndTheContentFollowsTheIndex(t *testing.T) {
	theme := gfx.LightTheme()
	current := 0
	draw := func(canvas *gfx.Canvas) (shown string) {
		var view View
		view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Gap(8).Children(func() {
				if i, ok := f.Tabs(tabsScope, tabLabels, current).TabClicked(); ok {
					current = i
				}
				switch current {
				case 0:
					shown = "general"
				case 1:
					shown = "keys"
				}
			})
		})
		return shown
	}
	canvas := testCanvas()
	if shown := draw(canvas); shown != "general" {
		t.Fatalf("shows %q at first", shown)
	}
	if len(canvas.Hits()) != 3 {
		t.Fatalf("%d tab hits, want 3", len(canvas.Hits()))
	}
	canvas = testCanvas()
	canvas.BeginEventPass(tabsScope.At(1), gfx.ClickInfo{})
	draw(canvas)
	canvas.EndEventPass()
	if current != 1 {
		t.Fatalf("a click on the second tab left %d selected", current)
	}
	if shown := draw(testCanvas()); shown != "keys" {
		t.Errorf("shows %q after the click", shown)
	}
}

func TestTabsAllocateNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Gap(8).Children(func() {
				f.Tabs(tabsScope, tabLabels, 1)
				f.Tabs(tabsScope, tabLabels, 2).Line()
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("warm tabs allocate %v times a frame, want 0", allocs)
	}
}
