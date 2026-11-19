package base

import (
	"github.com/vivaui/viva/gfx"
)

type TabsDecl struct {
	scope    gfx.Scope
	labels   []string
	icon     func(index int) gfx.Icon
	selected int
	line     bool
	vertical bool
}

// tabHeight is the height of one trigger of a components.Tabs; the line variant
// draws its two pixel indicator three pixels beyond the strip.
const (
	tabHeight    = 36
	TabIndicator = 5
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
func (f *Frame) Tabs(scope gfx.Scope, labels []string, selected int) Box {
	b := f.Box(kindTabs)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).tabs))
	WidgetsOf(f).tabs = append(WidgetsOf(f).tabs, TabsDecl{scope: scope, labels: labels, selected: selected})
	return b
}

// TabIcons puts an icon before every label of a Tabs strip; icon is asked for
// each tab's index and may return a zero icon to leave that tab text-only.
//
//	ui.Tabs(c, &m.tab, tabs, nil).TabIcons(func(i int) gfx.Icon { return icons[i] })
func (b Box) TabIcons(icon func(index int) gfx.Icon) Box {
	n := b.N()
	if n.Kind == kindTabs && icon != nil {
		WidgetsOf(b.f).tabs[n.Aux].icon = icon
	}
	return b
}

// Line makes a Tabs strip the transparent variant with an underline under the active
// tab. Vertical stacks its tabs in a column, with the line at their trailing edge.
func (b Box) Line() Box {
	if n := b.N(); n.Kind == kindTabs {
		WidgetsOf(b.f).tabs[n.Aux].line = true
	}
	return b
}

// Vertical stacks the tabs of a Tabs strip in a column, and makes a Slider a tall one
// (minimum at the bottom) instead of a wide one.
func (b Box) Vertical() Box {
	switch n := b.N(); n.Kind {
	case kindTabs:
		WidgetsOf(b.f).tabs[n.Aux].vertical = true
	case kindSlider:
		n.Natural = 1 // a vertical Slider: its thumb moves up and down
	}
	return b
}

// TabClicked reports which tab of a Tabs strip this paint is delivering a click on.
func (b Box) TabClicked() (int, bool) {
	n := b.N()
	if n.Kind != kindTabs || b.f.Surface == nil {
		return 0, false
	}
	return b.f.Surface.ClickedIn(WidgetsOf(b.f).tabs[n.Aux].scope)
}

func (f *Frame) measureTabs(n *Node) {
	t := &WidgetsOf(f).tabs[n.Aux]
	var widest float32
	for _, label := range t.labels {
		widest = max(widest, f.Surface.TextWidthWeight(label, f.Palette.Metrics.FontSizeM, gfx.WeightMedium))
	}
	count := float32(len(t.labels))
	var gap float32
	if t.line {
		gap = 4
	}
	between := gap * max(count-1, 0)
	switch {
	case t.vertical && t.line:
		n.Pref = [2]float32{widest + 16 + TabIndicator + 1, tabHeight*count + between}
	case t.vertical:
		n.Pref = [2]float32{widest + 16, tabHeight*count + between}
	case t.line:
		n.Pref = [2]float32{(widest+24)*count + between, tabHeight + TabIndicator}
	default:
		n.Pref = [2]float32{(widest + 24) * count, tabHeight}
	}
}
