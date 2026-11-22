package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintTooltip paints the Tooltip of the element the pointer rests on, over
// everything the frame painted.
func paintTooltip(f *Frame) {
	if f.Surface == nil {
		return
	}
	// The element a tooltip belongs to: the one the pointer rests on, or the one a finger
	// held (the press that opens it, then the element it stays for).
	target := f.Surface.Hovered()
	state, _ := f.GetTooltip().(*components.HoverTooltip)
	if event, _, ok := f.Surface.PointerPass(); ok && event.Device == gfx.PointerTouch && event.Kind == gfx.PointerDown &&
		event.Button == gfx.PointerButtonSecondary {
		target = event.Target
	} else if state != nil && !state.Holding().IsZero() {
		target = state.Holding()
	}
	if !f.HasTooltip(target) {
		// The pointer is on another frame's element (a page in a pane of this one),
		// whose frame paints its tip, or on none.
		f.SetTooltipValue(nil)
		return
	}
	if state == nil {
		state = new(components.HoverTooltip)
		f.SetTooltipValue(state)
	}
	state.Paint(f.Surface, f.Palette, f.GetBounds(), components.TooltipOptions{Layer: TooltipLayer, SideOf: f.TooltipSideOf},
		f.Palette.TooltipDelay)
}
