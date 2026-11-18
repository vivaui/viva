package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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
func (b Box) Tooltip(text string) Box {
	if b.n().ID.IsZero() {
		panic("ui: Tooltip needs an element with an identity: a control, or a container given Hit")
	}
	b.f.extra(b.i).tooltip = text
	b.f.tooltips = true
	b.f.paintTooltipFn = (*frame).paintTooltip
	return b
}

// TooltipSide is the edge of its element a Tooltip opens beside.
type TooltipSide = components.TooltipSide

// The sides TooltipOn opens a Tooltip on.
const (
	TooltipTop    = components.TooltipTop
	TooltipRight  = components.TooltipRight
	TooltipBottom = components.TooltipBottom
	TooltipLeft   = components.TooltipLeft
)

// TooltipOn opens the element's Tooltip beside side rather than below it: an
// information icon at the end of a label shows its tip to its right, where the
// tip covers neither the label nor the rows under it. Without room on side, the
// tip opens on the opposite one.
//
//	ui.Button(c, "").Icon(info).Tooltip("Applies to new terminals").TooltipOn(ui.TooltipRight)
func (b Box) TooltipOn(side TooltipSide) Box {
	if b.n().ID.IsZero() {
		panic("ui: TooltipOn needs an element with an identity: a control, or a container given Hit")
	}
	b.f.extra(b.i).tooltipSide = uint8(side) + 1
	return b
}

// paintTooltip paints the Tooltip of the element the pointer rests on, over
// everything the frame painted.
func (f *frame) paintTooltip() {
	if f.Surface == nil {
		return
	}
	if !f.hasTooltip(f.Surface.Hovered()) {
		// The pointer is on another frame's element (a page in a pane of this one),
		// whose frame paints its tip, or on none.
		f.tooltip = nil
		return
	}
	f.tooltipState().Paint(f.Surface, f.theme, f.bounds, components.TooltipOptions{Layer: TooltipLayer, SideOf: f.tooltipSideOf},
		components.HoverTooltipDelay)
}

// tooltipState is the tooltip state the layer above owns; the engine only keeps
// it between paints.
func (f *frame) tooltipState() *components.HoverTooltip {
	state, _ := f.tooltip.(*components.HoverTooltip)
	if state == nil {
		state = new(components.HoverTooltip)
		f.tooltip = state
	}
	return state
}

// tooltipSideOf is the side TooltipOn gave the element id, or below.
func (f *frame) tooltipSideOf(ID gfx.ID) TooltipSide {
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
func (f *frame) hasTooltip(ID gfx.ID) bool {
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
