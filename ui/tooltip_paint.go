package ui

import (
	"time"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type tooltipState struct {
	components.HoverTooltip
	text       gfx.HoverHint
	since      time.Duration
	suppressed bool
}

// paintTooltip paints the Tooltip of the element the pointer rests on, over
// everything the frame painted.
func paintTooltip(f *Frame) {
	if f.Surface == nil {
		return
	}
	if page := f.GetPage(); page != nil {
		if text := page.HoveredText(); text.Text != "" {
			state := frameTooltipState(f)
			if !state.text.SameText(text) {
				state.text, state.since, state.suppressed = text, f.Surface.Now(), false
				state.HoverTooltip = components.HoverTooltip{}
			}
			if page.PointerDown() || !f.Surface.Pressed().IsZero() {
				state.suppressed = true
			}
			if state.suppressed {
				return
			}
			if remaining := f.Palette.TooltipDelay - (f.Surface.Now() - state.since); remaining > 0 {
				f.Surface.RequestFrameIn(remaining)
				return
			}
			components.BeginTooltip(f.Surface, f.Palette, f.GetBounds(), text.Bounds,
				components.TooltipOptions{Layer: TooltipLayer, Side: base.HoverHintSide(text)}).Paint(text.Text)
			return
		}
	}
	if state, _ := f.GetTooltip().(*tooltipState); state != nil {
		state.text, state.suppressed = gfx.HoverHint{}, false
	}
	// The element a tooltip belongs to: the one the pointer rests on, or the one a finger
	// held (the press that opens it, then the element it stays for).
	target := f.Surface.Hovered()
	if event, _, ok := f.Surface.PointerPass(); ok && event.Device == gfx.PointerTouch && event.Kind == gfx.PointerDown &&
		event.Button == gfx.PointerButtonSecondary {
		target = event.Target
	} else if state, _ := f.GetTooltip().(*tooltipState); state != nil && !state.Holding().IsZero() {
		target = state.Holding()
	}
	if !f.HasTooltip(target) {
		// The pointer is on another frame's element (a page in a pane of this one),
		// whose frame paints its tip, or on none.
		f.SetTooltipValue(nil)
		return
	}
	frameTooltipState(f).Paint(f.Surface, f.Palette, f.GetBounds(), components.TooltipOptions{Layer: TooltipLayer, SideOf: f.TooltipSideOf},
		f.Palette.TooltipDelay)
}

// tooltipState is the tooltip state the layer above owns; the engine only keeps
// it between paints.
func frameTooltipState(f *Frame) *tooltipState {
	state, _ := f.GetTooltip().(*tooltipState)
	if state == nil {
		state = new(tooltipState)
		f.SetTooltipValue(state)
	}
	return state
}
