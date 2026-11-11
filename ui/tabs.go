package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type tabsDecl struct {
	scope    gfx.Scope
	labels   []string
	selected int
	line     bool
	vertical bool
}

// tabHeight is the height of one trigger of a components.Tabs; the line variant
// draws its two pixel indicator three pixels beyond the strip.
const (
	tabHeight    = 36
	tabIndicator = 5
)

// Tabs adds a tab strip painted by components.Tabs: the tabs are the items of scope,
// one per label in order, and selected is the index of the active one (negative for
// none). What the tab shows is the application's: declare the content after the
// strip, in the column or stack the strip is in, for the active index. The strip
// shares its width among its tabs; Line() draws the transparent variant with an
// underline under the active tab, Vertical() stacks the tabs; Disabled makes them inert.
//
//	if i, ok := f.Tabs(tabs, []string{"General", "Keys", "Advanced"}, m.tab).Line().TabClicked(); ok { m.tab = i }
//	switch m.tab { case 0: ... }
//
// The labels slice is kept until End. Arrow keys between tabs are components.TabsKeyDown's.
func (f *frame) Tabs(scope gfx.Scope, labels []string, selected int) Box {
	b := f.box(kindTabs)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.widgets.tabs))
	f.widgets.tabs = append(f.widgets.tabs, tabsDecl{scope: scope, labels: labels, selected: selected})
	return b
}

// Line makes a Tabs strip the transparent variant with an underline under the active
// tab. Vertical stacks its tabs in a column, with the line at their trailing edge.
func (b Box) Line() Box {
	if n := b.n(); n.kind == kindTabs {
		b.f.widgets.tabs[n.aux].line = true
	}
	return b
}

// Vertical stacks the tabs of a Tabs strip in a column, and makes a Slider a tall one
// (minimum at the bottom) instead of a wide one.
func (b Box) Vertical() Box {
	switch n := b.n(); n.kind {
	case kindTabs:
		b.f.widgets.tabs[n.aux].vertical = true
	case kindSlider:
		n.natural = 1 // a vertical Slider: its thumb moves up and down
	}
	return b
}

// TabClicked reports which tab of a Tabs strip this paint is delivering a click on.
func (b Box) TabClicked() (int, bool) {
	n := b.n()
	if n.kind != kindTabs || b.f.canvas == nil {
		return 0, false
	}
	return b.f.canvas.ClickedIn(b.f.widgets.tabs[n.aux].scope)
}

func (f *frame) measureTabs(n *node) {
	t := &f.widgets.tabs[n.aux]
	var widest float32
	for _, label := range t.labels {
		widest = max(widest, f.canvas.TextWidthWeight(label, f.theme.Metrics.FontSizeM, gfx.WeightMedium))
	}
	count := float32(len(t.labels))
	var gap float32
	if t.line {
		gap = 4
	}
	between := gap * max(count-1, 0)
	switch {
	case t.vertical && t.line:
		n.pref = [2]float32{widest + 16 + tabIndicator + 1, tabHeight*count + between}
	case t.vertical:
		n.pref = [2]float32{widest + 16, tabHeight*count + between}
	case t.line:
		n.pref = [2]float32{(widest+24)*count + between, tabHeight + tabIndicator}
	default:
		n.pref = [2]float32{(widest + 24) * count, tabHeight}
	}
}

func (f *frame) paintTabs(n *node) {
	t := &f.widgets.tabs[n.aux]
	bounds := Rect{X: n.rect.X, Y: n.rect.Y, Width: n.rect.Width, Height: tabHeight}
	if t.vertical {
		bounds.Height = n.rect.Height
		if t.line {
			bounds.Width = max(bounds.Width-tabIndicator-1, 0)
		}
	}
	tabs := components.BeginTabList(f.canvas, f.theme, bounds, t.scope, t.labels, t.selected)
	if t.line {
		tabs = tabs.Line()
	}
	if t.vertical {
		tabs = tabs.Vertical()
	}
	if n.flags&flagDisabled != 0 {
		tabs = tabs.Disabled()
	}
	tabs.Paint()
}
