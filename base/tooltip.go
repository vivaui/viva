package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// TooltipLayer is the canvas layer tooltips are painted on: over the page, its
// popovers, menus and dialogs.
const TooltipLayer = 1000

// OverflowTooltip supplies the complete value when an application shortened
// the displayed text itself, possibly across several highlighted labels. Call
// it only when content was omitted. Unlike Tooltip it declares no control and
// leaves the enclosing row's pointer, focus and accessibility unchanged. Giving
// it is the caller's opt-in, so it shows whatever PageConfig.EllipsisTooltips
// says. Text truncated by Ellipsis or MaxLines supplies its complete value
// itself where the page or an EllipsisTooltip turned the tooltips on.
func (b Box) OverflowTooltip(text string) Box {
	b.f.Extra(b.i).overflowTip = text
	return b
}

// HoveredText returns this page's visible truncation hint under the pointer, as
// the page's last paint or pointer event found it.
func (p *Page) HoveredText() gfx.HoverHint {
	if p == nil {
		return gfx.HoverHint{}
	}
	return p.hoverText
}

// hoverTextOn looks up the page's hint under the pointer among those surface
// holds. A page looks once per paint, before the tooltip, and once per pointer
// event; HoveredText returns what it found.
func (p *Page) hoverTextOn(surface *gfx.Canvas) gfx.HoverHint {
	if !p.pointerIn || surface == nil {
		return gfx.HoverHint{}
	}
	return surface.HoverHintAt(p.root, p.pointer.X, p.pointer.Y)
}

// PointerDown reports a held pointer button, including a press on plain text.
func (p *Page) PointerDown() bool { return p != nil && p.pointerDown }

// Tooltip names what the element does in a tip that appears beside it while the
// pointer rests on it, and is its description for assistive technology: what an
// icon-only button needs. The tip hides on a press and stays hidden until the
// pointer leaves. The element is a control, or a container given an identity.
// While the pointer is over shortened text inside the element whose complete
// value EllipsisTooltip or OverflowTooltip shows, that tip shows instead.
//
//	ui.Button(c, "").Icon(trash).Tooltip("Delete")
func (b Box) Tooltip(Text string) Box {
	if b.N().ID.IsZero() {
		panic("ui: Tooltip needs an element with an identity: a control, or a container given Hit")
	}
	b.f.Extra(b.i).tooltip = Text
	b.f.tooltips = true
	b.f.paintTooltipFn = (*Frame).paintTooltip
	return b
}

// TooltipSide is the edge of its element a Tooltip opens beside.
type TooltipSide = style.TooltipSide

// The sides TooltipOn opens a Tooltip on.
const (
	TooltipTop    = style.TooltipTop
	TooltipRight  = style.TooltipRight
	TooltipBottom = style.TooltipBottom
	TooltipLeft   = style.TooltipLeft
)

// TooltipOn opens the element's Tooltip beside side rather than below it: an
// information icon at the end of a label shows its tip to its right, where the
// tip covers neither the label nor the rows under it. Without room on side, the
// tip opens on the opposite one.
//
//	ui.Button(c, "").Icon(info).Tooltip("Applies to new terminals").TooltipOn(ui.TooltipRight)
func (b Box) TooltipOn(side TooltipSide) Box {
	if b.N().ID.IsZero() {
		panic("ui: TooltipOn needs an element with an identity: a control, or a container given Hit")
	}
	b.f.Extra(b.i).tooltipSide = uint8(side) + 1
	return b
}

// tooltipSideOf is the side TooltipOn gave the element id, or below.
func (f *Frame) TooltipSideOf(ID gfx.ID) TooltipSide {
	for i := range f.Nodes {
		if n := &f.Nodes[i]; n.ID == ID && n.Ext != 0 && f.Exts[n.Ext].tooltipSide != 0 {
			return TooltipSide(f.Exts[n.Ext].tooltipSide - 1)
		}
	}
	return TooltipBottom
}

// hasTooltip reports whether id names an element of this frame given a Tooltip.
// A Memo declares what it holds again while the pointer is on it, so the element
// under the pointer is always one the frame declared.
func (f *Frame) HasTooltip(ID gfx.ID) bool {
	if ID.IsZero() {
		return false
	}
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.ID == ID && n.Ext != 0 && f.Exts[n.Ext].tooltip != "" {
			return true
		}
	}
	return false
}
