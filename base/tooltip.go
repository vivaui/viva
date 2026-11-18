package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// TooltipLayer is the canvas layer tooltips are painted on: over the page, its
// popovers, menus and dialogs.
const TooltipLayer = 1000

// Tooltip names what the element does in a tip that appears beside it while the
// pointer rests on it, and is its description for assistive technology: what an
// icon-only button needs. The tip hides on a press and stays hidden until the
// pointer leaves. The element is a control, or a container given an identity.
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
