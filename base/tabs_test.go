package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"

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
		Name     string
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
		sameCanvas(t, test.Name, flexed, direct)
	}
}

// A Tabs given a height paints its triggers that tall, so a strip fits the row
// sized for it and a line's indicator keeps its room below them.
func TestTabsTakeTheirDeclaredHeight(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		Name    string
		declare func(Box) Box
		line    bool
		height  float32
	}{
		{"28", func(b Box) Box { return b }, false, 28},
		{"32", func(b Box) Box { return b }, false, 32},
		{"34", func(b Box) Box { return b }, false, 34},
		{"line-40", func(b Box) Box { return b.Line() }, true, 40},
	} {
		flexed, direct := testCanvas(), testCanvas()
		var view View
		view.Draw(flexed, theme, gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Children(func() { test.declare(f.Tabs(tabsScope, tabLabels, 1).Height(test.height)) })
		})
		stripHeight := test.height
		if test.line {
			// A line's height keeps the indicator's room below its triggers.
			stripHeight -= TabIndicator
		}
		strip := components.BeginTabList(direct, theme, gfx.Rect{Width: 300, Height: stripHeight}, tabsScope, tabLabels, 1)
		if test.line {
			strip = strip.Line()
		}
		strip.Paint()
		sameCanvas(t, test.Name, flexed, direct)
	}
}

func TestTabsReportTheClickedTabAndTheContentFollowsTheIndex(t *testing.T) {
	theme := gfx.LightTheme()
	current := 0
	draw := func(Surface *gfx.Canvas) (shown string) {
		var view View
		view.Draw(Surface, theme, gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
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

// A strip too narrow for its labels paints only the icons TabIcons gives it,
// while the labels remain the names a tab is reached and read by.
func TestIconOnlyTabsPaintTheirIconsAndKeepTheirNames(t *testing.T) {
	theme := gfx.LightTheme()
	icons := []gfx.Icon{
		{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 4), gfx.LineTo(20, 20)}},
		{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 12), gfx.LineTo(20, 12)}},
		{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(12, 4), gfx.LineTo(12, 20)}},
	}
	flexed, direct := testCanvas(), testCanvas()
	var view View
	view.Draw(flexed, theme, gfx.Rect{Width: 40, Height: 108}, func(f *Frame) {
		f.Column().Children(func() {
			f.Tabs(tabsScope, tabLabels, 1).
				TabIcons(func(index int) gfx.Icon { return icons[index] }).
				TabIconOnly().Vertical().Height(108)
		})
	})
	options := make([]TabOption, len(tabLabels))
	for index, label := range tabLabels {
		options[index] = TabOption{ID: tabsScope.At(index), Label: label, Icon: icons[index], IconOnly: true}
	}
	components.BeginTabs(direct, theme, gfx.Rect{Width: 40, Height: 108}, options, options[1].ID).Vertical().Paint()
	sameCanvas(t, "icon-only", flexed, direct)
	if texts := flexed.Texts(); len(texts) != 0 {
		t.Fatalf("an icon-only strip painted %d labels, want none", len(texts))
	}
	for index, label := range tabLabels {
		if hit := hitByID(flexed, tabsScope.At(index)); hit.Label != label || hit.Hint != label {
			t.Errorf("tab %d is named %q and hinted %q, want %q", index, hit.Label, hit.Hint, label)
		}
	}
}

// A tab of an icon-only strip whose icon is zero keeps its label: the strip
// would otherwise show an empty trigger.
func TestAnIconOnlyTabWithoutAnIconKeepsItsLabel(t *testing.T) {
	theme := gfx.LightTheme()
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 4), gfx.LineTo(20, 20)}}
	canvas := testCanvas()
	var view View
	view.Draw(canvas, theme, gfx.Rect{Width: 40, Height: 108}, func(f *Frame) {
		f.Column().Children(func() {
			f.Tabs(tabsScope, tabLabels, 1).
				TabIcons(func(index int) gfx.Icon {
					if index == 0 {
						return gfx.Icon{}
					}
					return icon
				}).TabIconOnly().Vertical().Height(108)
		})
	})
	texts := canvas.Texts()
	if len(texts) != 1 || texts[0].Value != tabLabels[0] {
		t.Fatalf("texts = %v, want only %q", texts, tabLabels[0])
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
