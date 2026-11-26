package base

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

// What EllipsisTooltip and EllipsisTooltipOn set on an element, in one byte so
// that painting checks a single field. The low bits are the switch, ellipsisTipOn
// or ellipsisTipOff; the bits above are 1 + the side EllipsisTooltipOn gave.
// Zero in either part follows what the element is inside.
const (
	ellipsisTipOn uint8 = 1 + iota
	ellipsisTipOff

	ellipsisTipSwitch    uint8 = 3
	ellipsisTipSideShift       = 2
)

// ellipsisState is what the text being painted does with the tooltips of
// shortened text: whether EllipsisTooltip's show, and 1 + the side every one
// of them opens on, OverflowTooltip's too; a zero side opens them below.
type ellipsisState struct {
	on   bool
	side uint8
}

// popupFrom is the element a Popup was declared in, which it takes no room in
// but follows for EllipsisTooltip when it hangs off no element.
type popupFrom struct{ popup, from int32 }

// EllipsisTooltip turns on, or with false off, the tooltip that shows the
// complete value of text shortened with an ellipsis, for the element and what
// it holds: a Text cut by Ellipsis or MaxLines, and the labels components
// shorten themselves, such as Table cells, Select values and menu items. It
// overrides PageConfig.EllipsisTooltips and an EllipsisTooltip further out. A
// Popup follows the element it hangs off (BelowBox and the like) or, placed by
// a rectangle or a point, the element it was declared in.
//
//	ui.Text(c, path).Ellipsis().EllipsisTooltip(true)
//	ui.Table(c, &table, spec).EllipsisTooltip(true)
func (b Box) EllipsisTooltip(on bool) Box {
	b.N()
	tip := ellipsisTipOff
	if on {
		tip = ellipsisTipOn
	}
	x := b.f.Extra(b.i)
	x.ellipsisTip = x.ellipsisTip&^ellipsisTipSwitch | tip
	b.f.ellipsisSet = true
	return b
}

// EllipsisTooltipOn opens the tooltips of the shortened text in the element and
// what it holds beside side of the text rather than below it, as TooltipOn does
// for a Tooltip: those EllipsisTooltip shows and those OverflowTooltip gives. A
// list in a sidebar on the window's left opens them to its right, over the
// content beside it, and a status bar along the window's bottom opens them
// above it. It does not turn them on. The nearest EllipsisTooltipOn decides, and
// a Popup follows it as it follows EllipsisTooltip. Without room on side, a
// tooltip opens on the opposite one.
//
//	ui.Column(c).EllipsisTooltipOn(ui.TooltipRight).Children(func() { … })
func (b Box) EllipsisTooltipOn(side TooltipSide) Box {
	b.N()
	x := b.f.Extra(b.i)
	x.ellipsisTip = x.ellipsisTip&ellipsisTipSwitch | (uint8(side)+1)<<ellipsisTipSideShift
	b.f.ellipsisSet = true
	return b
}

// under is the state inside an element given set, from state outside it.
func (state ellipsisState) under(set uint8) ellipsisState {
	if tip := set & ellipsisTipSwitch; tip != 0 {
		state.on = tip == ellipsisTipOn
	}
	if side := set >> ellipsisTipSideShift; side != 0 {
		state.side = side
	}
	return state
}

// setEllipsis sets what is painted next does with the tooltips of shortened
// text, and returns what it did. Components see it on the canvas: as the hover
// hint owner, which is zero while EllipsisTooltip's are off, and the hint side.
func (f *Frame) setEllipsis(state ellipsisState) ellipsisState {
	was := f.ellipsis
	if state != was && f.page != nil && f.Surface != nil {
		f.ellipsis = state
		if state.on != was.on {
			var owner gfx.Scope
			if state.on {
				owner = f.page.root
			}
			f.Surface.SetHoverHintOwner(owner)
		}
		if state.side != was.side {
			f.Surface.SetHoverHintSide(state.side)
		}
	}
	return was
}

// restoreHoverHints gives the canvas back the hover hint owner and side it had
// before the frame painted, those of the page whose pane holds this one.
func (f *Frame) restoreHoverHints(owner gfx.Scope, side uint8) {
	f.Surface.SetHoverHintOwner(owner)
	f.Surface.SetHoverHintSide(side)
}

// ellipsisAt is the state element i paints in: the nearest EllipsisTooltip and
// EllipsisTooltipOn on it or on what it is inside, or else the page's setting.
func (f *Frame) ellipsisAt(i int32) ellipsisState {
	var tip, side uint8
	for steps := 0; i >= 0 && steps < len(f.Nodes) && (tip == 0 || side == 0); steps++ {
		n := &f.Nodes[i]
		set := f.Exts[n.Ext].ellipsisTip
		if tip == 0 {
			tip = set & ellipsisTipSwitch
		}
		if side == 0 {
			side = set >> ellipsisTipSideShift
		}
		switch {
		case n.Parent >= 0:
			i = n.Parent
		case n.AnchorRef > 0:
			i = n.AnchorRef - 1
		default:
			i = f.popupParent(i)
		}
	}
	state := ellipsisState{on: f.page != nil && f.page.config.EllipsisTooltips}
	return state.under(tip | side<<ellipsisTipSideShift)
}

// popupParent is the element Popup i was declared in, or -1 for the root and
// for a popup declared outside every container.
func (f *Frame) popupParent(i int32) int32 {
	at, found := slices.BinarySearchFunc(f.popupFrom, i, func(p popupFrom, i int32) int {
		return int(p.popup - i)
	})
	if !found {
		return -1
	}
	return f.popupFrom[at].from
}
