package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Popover adds a non-modal panel that hangs off anchor, as shadcn's Popover: a
// surface with the popover look, its children declared with Children as in any
// container, placed by the anchor once the page is laid out and painted over the
// page. A press outside it or Escape reports Dismissed, and the press goes on to
// what is under the pointer; a press on the anchor only closes it, so a toggle does
// not open it again. Declare it where it is open; the focus goes back to where it
// was as it closes.
//
//	filter := f.Button(filterID, "Filter").Outline()
//	if filter.Clicked() { m.open = !m.open }
//	if m.open {
//		p := f.Popover(filter, ui.PopoverSpec{Scope: popScope, DismissID: dismissID}).Children(func() { ... })
//		if p.Dismissed() { m.open = false }
//	}
func (f *frame) Popover(Anchor Box, spec PopoverSpec) Box {
	metrics := f.Palette.Metrics
	look := components.PopoverRecipe(f.Palette)
	width := spec.Width
	if width <= 0 {
		width = 288
	}
	offset := spec.Offset
	if offset == 0 {
		offset = 4
	}
	align := spec.Align
	if align == Auto {
		align = Center
	}
	b := f.Popup()
	switch spec.Side {
	case components.PopoverTop:
		b.AboveBox(Anchor, offset, align)
	case components.PopoverRight:
		b.BesideBox(Anchor, offset, align)
	case components.PopoverLeft:
		b.BesideLeftBox(Anchor, offset, align)
	default:
		b.BelowBox(Anchor, offset, align)
	}
	b.n().PlaceShift = spec.AlignOffset
	b.Width(width).Pad(metrics.PadSurface).Gap(metrics.GapItem).
		Background(look.Fill).Border(1, look.Border).Radius(look.Radius).Shadow(look.Shadow)
	// Within the frame less the margin the components keep from its edges.
	b.Within(f.bounds.Inset(gfx.All(8)))
	n := b.n()
	n.Flags |= flagPopover
	f.paintPopoverFn = (*frame).paintPopover
	n.Aux = int32(len(widgetsOf(f).popovers))
	widgetsOf(f).popovers = append(widgetsOf(f).popovers, popoverDecl{spec: spec, Anchor: Anchor.i, layer: widgetsOf(f).layer})
	return b
}

// paintPopover paints a Popover's root inside the layer and focus scope it opens: the
// press outside, the anchor's press and Escape go to its DismissID.
func (f *frame) paintPopover(i int32, n *node) {
	d := &widgetsOf(f).popovers[n.Aux]
	spec := &d.spec
	canvas := f.Surface
	layer := max(d.layer, f.popupLayer, widgetLayerBase) + 2
	canvas.PushLayer(layer)
	canvas.PushFocusScope(spec.Scope)
	canvas.ReturnFocus(spec.Scope)
	if !spec.InitialFocusID.IsZero() && !widgetsOf(f).popoverAsked(popoverFocusAskFor(spec)) {
		// As AutoFocus is: the keyboard is asked for as the panel opens, not on
		// every paint, so moving the focus inside the panel stays where the user
		// put it.
		canvas.RequestFocus(spec.InitialFocusID)
		widgetsOf(f).popoverFocus = append(widgetsOf(f).popoverFocus, popoverFocusAskFor(spec))
	}
	if !spec.DismissID.IsZero() {
		pass := !spec.ConsumeOutsidePress
		canvas.HitWithOptions(spec.DismissID, f.bounds, "Dismiss popover", gfx.HitOptions{
			Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassPrimary: pass, PassSecondary: pass,
		})
		if !spec.KeepOpenOnAnchor {
			canvas.HitWithOptions(spec.DismissID, f.Nodes[d.Anchor].Rect, "Dismiss popover", gfx.HitOptions{
				Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: pass,
			})
		}
	}
	// The panel is opaque to the pointer: a press on its padding does not reach the page.
	canvas.Block(n.Rect)
	savedLayer := widgetsOf(f).layer
	widgetsOf(f).layer = layer
	n.Flags &^= flagPopover
	presence, pivot := components.PopupMotion(f.Palette, n.Rect, f.Nodes[d.Anchor].Rect)
	canvas.BeginPresence(spec.Scope, 0, presence, pivot)
	f.paint(i)
	canvas.EndPresence()
	f.Nodes[i].Flags |= flagPopover
	widgetsOf(f).layer = savedLayer
	if !spec.DismissID.IsZero() {
		canvas.SetEscapeTarget(spec.DismissID)
	}
	canvas.PopFocusScope()
	canvas.PopLayer()
}
