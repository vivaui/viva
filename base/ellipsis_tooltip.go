package base

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

// What EllipsisTooltip set on an element; zero is what the element is inside.
const (
	ellipsisTipOn uint8 = 1 + iota
	ellipsisTipOff
)

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
	b.f.Extra(b.i).ellipsisTip = tip
	b.f.ellipsisSet = true
	return b
}

// setEllipsisTips sets whether what is painted next shows the tooltips
// EllipsisTooltip describes, and returns what it was. Components see it as the
// canvas's hover hint owner, which is zero while it is off.
func (f *Frame) setEllipsisTips(on bool) bool {
	was := f.ellipsisTips
	if on != was && f.page != nil && f.Surface != nil {
		f.ellipsisTips = on
		var owner gfx.Scope
		if on {
			owner = f.page.root
		}
		f.Surface.SetHoverHintOwner(owner)
	}
	return was
}

// ellipsisTipsAt reports whether element i shows the tooltips: the nearest
// EllipsisTooltip on it or on what it is inside, or else the page's setting.
func (f *Frame) ellipsisTipsAt(i int32) bool {
	for steps := 0; i >= 0 && steps < len(f.Nodes); steps++ {
		n := &f.Nodes[i]
		if tip := f.Exts[n.Ext].ellipsisTip; tip != 0 {
			return tip == ellipsisTipOn
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
	return f.page != nil && f.page.config.EllipsisTooltips
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
