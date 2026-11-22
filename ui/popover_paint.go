package ui

import (
	"github.com/vivaui/viva/base"
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
//	if m.GetOpen() {
//		p := f.Popover(filter, ui.PopoverSpec{Scope: popScope, DismissID: dismissID}).Children(func() { ... })
//		if p.Dismissed() { m.open = false }
//	}
func framePopover(f *Frame, Anchor Box, spec PopoverSpec) Box {
	metrics := f.Palette.Metrics
	look := components.PopoverRecipe(f.Palette)
	if spec.Radius > 0 {
		look.Radius = spec.Radius
	}
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
	b.N().PlaceShift = spec.AlignOffset
	padding := spec.Padding
	if padding <= 0 {
		padding = metrics.PadSurface
	}
	b.Width(width).Pad(padding).Gap(metrics.GapItem).
		Background(look.Fill).Border(1, look.Border).Radius(look.Radius).Shadow(look.Shadow)
	// Within the frame less the margin the components keep from its edges.
	margin := spec.Margin
	if margin <= 0 {
		margin = 8
	}
	b.Within(f.GetBounds().Inset(gfx.All(margin)))
	n := b.N()
	n.Flags |= FlagPopover
	f.SetPaintPopoverFn(paintPopover)
	n.Aux = int32(len(base.WidgetsOf(f).GetPopovers()))
	base.WidgetsOf(f).SetPopovers(append(base.WidgetsOf(f).GetPopovers(), base.NewPopoverDecl(spec, Anchor.GetI(), base.WidgetsOf(f).GetLayer())))
	return b
}

// paintPopover paints a Popover's root inside the layer and focus scope it opens: the
// press outside, the anchor's press and Escape go to its DismissID.
func paintPopover(f *Frame, i int32, n *Node) {
	d := &base.WidgetsOf(f).GetPopovers()[n.Aux]
	spec := d.SpecPtr()
	canvas := f.Surface
	layer := max(d.GetLayer(), f.GetPopupLayer(), widgetLayerBase) + 2
	canvas.PushLayer(layer)
	canvas.PushFocusScope(spec.Scope)
	canvas.ReturnFocus(spec.Scope)
	if !spec.InitialFocusID.IsZero() && !base.WidgetsOf(f).PopoverAsked(PopoverFocusAskFor(spec)) {
		// As AutoFocus is: the keyboard is asked for as the panel opens, not on
		// every paint, so moving the focus inside the panel stays where the user
		// put it.
		canvas.RequestFocus(spec.InitialFocusID)
		base.WidgetsOf(f).SetPopoverFocus(append(base.WidgetsOf(f).GetPopoverFocus(), PopoverFocusAskFor(spec)))
	}
	if !spec.DismissID.IsZero() {
		pass := !spec.ConsumeOutsidePress
		canvas.HitWithOptions(spec.DismissID, f.GetBounds(), "Dismiss popover", gfx.HitOptions{
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
	savedLayer := base.WidgetsOf(f).GetLayer()
	base.WidgetsOf(f).SetLayer(layer)
	n.Flags &^= FlagPopover
	if !spec.Instant {
		presence, pivot := components.PopupMotion(f.Palette, n.Rect, f.Nodes[d.Anchor].Rect)
		canvas.BeginPresence(spec.Scope, 0, presence, pivot)
	}
	f.PaintChild(i)
	if !spec.Instant {
		canvas.EndPresence()
	}
	f.Nodes[i].Flags |= FlagPopover
	base.WidgetsOf(f).SetLayer(savedLayer)
	if !spec.DismissID.IsZero() {
		canvas.SetEscapeTarget(spec.DismissID)
	}
	canvas.PopFocusScope()
	canvas.PopLayer()
}
