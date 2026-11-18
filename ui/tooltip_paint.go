package ui

import (
	"github.com/vivaui/viva/components"
)

// paintTooltip paints the Tooltip of the element the pointer rests on, over
// everything the frame painted.
func paintTooltip(f *Frame) {
	if f.Surface == nil {
		return
	}
	if !f.HasTooltip(f.Surface.Hovered()) {
		// The pointer is on another frame's element (a page in a pane of this one),
		// whose frame paints its tip, or on none.
		f.SetTooltipValue(nil)
		return
	}
	frameTooltipState(f).Paint(f.Surface, f.Palette, f.GetBounds(), components.TooltipOptions{Layer: TooltipLayer, SideOf: f.TooltipSideOf},
		components.HoverTooltipDelay)
}

// tooltipState is the tooltip state the layer above owns; the engine only keeps
// it between paints.
func frameTooltipState(f *Frame) *components.HoverTooltip {
	state, _ := f.GetTooltip().(*components.HoverTooltip)
	if state == nil {
		state = new(components.HoverTooltip)
		f.SetTooltipValue(state)
	}
	return state
}
