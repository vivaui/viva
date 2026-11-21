package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

type TabsDecl struct {
	scope    gfx.Scope
	labels   []string
	icon     func(index int) gfx.Icon
	selected int
	line     bool
	vertical bool
	iconOnly bool
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
	n.Flags |= flagThemed
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

// TabIconOnly hides the labels of a Tabs strip and paints only the icons that
// TabIcons gives it, for a strip too narrow to hold its labels. The labels stay
// the tabs' accessible names and their hover hints, as components.TabOption.IconOnly
// keeps them. A tab whose icon function returns a zero icon keeps its label, since
// there would be nothing to see otherwise.
//
//	ui.Tabs(c, &m.tab, tabs, nil).TabIcons(icons).TabIconOnly().Vertical()
func (b Box) TabIconOnly() Box {
	if n := b.N(); n.Kind == kindTabs {
		WidgetsOf(b.f).tabs[n.Aux].iconOnly = true
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
	_, listPadding, triggerHeight, triggerPad, lineGap, fontSize, weight := style.TabsGeometry(f.Palette.Metrics)
	if n.FontSize > 0 {
		fontSize = n.FontSize
	}
	if n.WeightSet || n.Weight != 0 {
		weight = n.Weight
	}
	// The recipe filled every side the caller did not set with the same theme
	// padding, so the element's Pad is the strip's four inner edges here.
	top, right, bottom, left := n.Pad[0], n.Pad[1], n.Pad[2], n.Pad[3]
	if n.PadSet&padTop == 0 {
		top = listPadding
	}
	if n.PadSet&padRight == 0 {
		right = listPadding
	}
	if n.PadSet&padBottom == 0 {
		bottom = listPadding
	}
	if n.PadSet&padLeft == 0 {
		left = listPadding
	}
	var widest float32
	for index, label := range t.labels {
		width := f.Surface.TextWidthWeight(label, fontSize, weight)
		if t.icon != nil {
			icon := t.icon(index)
			if len(icon.Commands) > 0 || len(icon.Layers) > 0 {
				if t.iconOnly {
					width = fontSize
				} else {
					width += fontSize + 6
				}
			}
		}
		widest = max(widest, width)
	}
	count := float32(len(t.labels))
	gap := float32(0)
	if n.Gap != 0 {
		gap = n.Gap
	} else if t.line {
		gap = lineGap
	}
	between := gap * max(count-1, 0)
	triggerWidth := widest + 2*triggerPad
	switch {
	case t.vertical && t.line:
		n.Pref = [2]float32{triggerWidth + left + right + TabIndicator + 1, triggerHeight*count + between + top + bottom}
	case t.vertical:
		n.Pref = [2]float32{triggerWidth + left + right, triggerHeight*count + between + top + bottom}
	case t.line:
		n.Pref = [2]float32{triggerWidth*count + between + left + right, top + triggerHeight + bottom + TabIndicator}
	default:
		n.Pref = [2]float32{triggerWidth*count + between + left + right, top + triggerHeight + bottom}
	}
}
