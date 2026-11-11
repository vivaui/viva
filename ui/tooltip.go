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
	if b.n().id.IsZero() {
		panic("ui: Tooltip needs an element with an identity: a control, or a container given Hit")
	}
	b.f.extra(b.i).tooltip = text
	b.f.tooltips = true
	b.f.paintTooltipFn = (*Frame).paintTooltip
	return b
}

// paintTooltip paints the Tooltip of the element the pointer rests on, over
// everything the frame painted.
func (f *Frame) paintTooltip() {
	if f.canvas == nil {
		return
	}
	if !f.hasTooltip(f.canvas.Hovered()) {
		// The pointer is on another frame's element (a page in a pane of this one),
		// whose frame paints its tip, or on none.
		f.tooltip = components.HoverTooltip{}
		return
	}
	f.tooltip.Paint(f.canvas, f.theme, f.bounds, components.TooltipOptions{Layer: TooltipLayer},
		f.theme.TooltipDelay)
}

// hasTooltip reports whether id names an element of this frame given a Tooltip.
// A Memo declares what it holds again while the pointer is on it, so the element
// under the pointer is always one the frame declared.
func (f *Frame) hasTooltip(id gfx.ID) bool {
	if id.IsZero() {
		return false
	}
	for i := range f.nodes {
		n := &f.nodes[i]
		if n.id == id && n.ext != 0 && f.exts[n.ext].tooltip != "" {
			return true
		}
	}
	return false
}
