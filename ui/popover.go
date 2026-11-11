package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// PopoverSpec declares a Popover: its identities, which are the application's, and
// where it goes.
type PopoverSpec struct {
	// Scope is the focus scope of the panel: Tab moves among its controls, and the
	// focus goes back to where it was when the panel closes. DismissID is the target
	// of a press outside the panel, of Escape and, unless KeepOpenOnAnchor, of a press
	// on the anchor: Dismissed reports it.
	Scope, DismissID gfx.ID
	// InitialFocusID, when set, takes the keyboard as the panel opens. It is asked
	// once, as AutoFocus is, and not again while the panel stays open, so the user
	// can move the focus inside it without the next paint pulling it back.
	InitialFocusID gfx.ID
	// Side is the edge of the anchor the panel opens on (below it unless set); it
	// flips to the opposite edge when there is no room. Align lines the panel up with
	// the anchor along that edge (Auto is Center). Offset is the gap to the anchor,
	// 4 when zero.
	Side   components.PopoverSide
	Align  Align
	Offset float32
	// Title and Description, when set, head the panel as shadcn's PopoverHeader
	// does: the title in the popover foreground, medium weight, and under it the
	// description in the muted color, wrapped to the width. The body follows.
	Title, Description string
	// TitleWeight is the title's font weight, medium when zero, as
	// components.PopoverOptions.TitleWeight is.
	TitleWeight float32
	// Width is the panel's width, 288 when zero; its height fits what is in it
	// (give the body a Height to fix it).
	Width float32
	// ConsumeOutsidePress keeps the press that closes the panel from also reaching
	// the control under it, which it does by default, as a web popover's does.
	ConsumeOutsidePress bool
	// KeepOpenOnAnchor lets a press on the anchor through to it instead of closing
	// the panel, for an anchor that is not the panel's toggle.
	KeepOpenOnAnchor bool
}

type popoverDecl struct {
	spec   PopoverSpec
	anchor int32
	layer  uint16 // the layer the Popover was declared in
}

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
func (f *frame) Popover(anchor Box, spec PopoverSpec) Box {
	metrics := f.theme.Metrics
	look := components.PopoverRecipe(f.theme)
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
		b.AboveBox(anchor, offset, align)
	case components.PopoverRight:
		b.BesideBox(anchor, offset, align)
	case components.PopoverLeft:
		b.BesideLeftBox(anchor, offset, align)
	default:
		b.BelowBox(anchor, offset, align)
	}
	b.Width(width).Pad(metrics.PadSurface).Gap(metrics.GapItem).
		Background(look.Fill).Border(1, look.Border).Radius(look.Radius).Shadow(look.Shadow)
	// Within the frame less the margin the components keep from its edges.
	b.Within(f.bounds.Inset(gfx.All(8)))
	n := b.n()
	n.flags |= flagPopover
	f.paintPopoverFn = (*frame).paintPopover
	n.aux = int32(len(f.widgets.popovers))
	f.widgets.popovers = append(f.widgets.popovers, popoverDecl{spec: spec, anchor: anchor.i, layer: f.widgets.layer})
	return b
}

// paintPopover paints a Popover's root inside the layer and focus scope it opens: the
// press outside, the anchor's press and Escape go to its DismissID.
func (f *frame) paintPopover(i int32, n *node) {
	d := &f.widgets.popovers[n.aux]
	spec := &d.spec
	canvas := f.canvas
	layer := max(d.layer, f.popupLayer, widgetLayerBase) + 2
	canvas.PushLayer(layer)
	canvas.PushFocusScope(spec.Scope)
	canvas.ReturnFocus(spec.Scope)
	if !spec.InitialFocusID.IsZero() && !f.widgets.popoverAsked(popoverFocusAskFor(spec)) {
		// As AutoFocus is: the keyboard is asked for as the panel opens, not on
		// every paint, so moving the focus inside the panel stays where the user
		// put it.
		canvas.RequestFocus(spec.InitialFocusID)
		f.widgets.popoverFocus = append(f.widgets.popoverFocus, popoverFocusAskFor(spec))
	}
	if !spec.DismissID.IsZero() {
		pass := !spec.ConsumeOutsidePress
		canvas.HitWithOptions(spec.DismissID, f.bounds, "Dismiss popover", gfx.HitOptions{
			Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassPrimary: pass, PassSecondary: pass,
		})
		if !spec.KeepOpenOnAnchor {
			canvas.HitWithOptions(spec.DismissID, f.nodes[d.anchor].rect, "Dismiss popover", gfx.HitOptions{
				Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: pass,
			})
		}
	}
	// The panel is opaque to the pointer: a press on its padding does not reach the page.
	canvas.Block(n.rect)
	savedLayer := f.widgets.layer
	f.widgets.layer = layer
	n.flags &^= flagPopover
	presence, pivot := components.PopupMotion(f.theme, n.rect, f.nodes[d.anchor].rect)
	canvas.BeginPresence(spec.Scope, 0, presence, pivot)
	f.paint(i)
	canvas.EndPresence()
	f.nodes[i].flags |= flagPopover
	f.widgets.layer = savedLayer
	if !spec.DismissID.IsZero() {
		canvas.SetEscapeTarget(spec.DismissID)
	}
	canvas.PopFocusScope()
	canvas.PopLayer()
}
