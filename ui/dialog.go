package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// DialogSpec declares a Dialog. The identities are the application's, as with
// components.DialogOptions.
type DialogSpec struct {
	Title, Description string
	// Width is the surface's width, 448 when zero (kept 32 pixels inside the frame).
	// Height is its height; zero fits the body.
	Width, Height float32
	// Layer is the compositing layer of the scrim and surface; zero is two above the
	// layer the Dialog is declared in, so a Select opened inside a dialog is over it.
	Layer uint16
	// Scope is the focus scope Tab stays inside; DismissID is the target of the
	// scrim, so a press outside the surface reports it (and Escape does); CloseID is
	// the close button's, and Escape's target when there is no DismissID.
	// InitialFocusID takes focus when the dialog opens.
	Scope, DismissID, CloseID, InitialFocusID gfx.ID
	// NoClose leaves out the close button in the corner.
	NoClose bool
}

type dialogDecl struct {
	spec                 DialogSpec
	header, body, footer int32
	layer                uint16
}

// Dialog adds a modal dialog painted by components.Dialog: a scrim over the whole
// frame, a centered surface with the title, the description and a close button, a
// body and a footer of buttons against the right edge. Declare it where it is open
// (the application decides whether to declare it at all); it takes no room in its
// container and is painted over everything. Body and Footer declare what is inside:
// ordinary children, laid out in the exact rectangles components.Dialog gives
// them, so the surface is as tall as its body needs.
//
//	if m.confirm {
//		d := f.Dialog(ui.DialogSpec{Title: "Delete host?", Description: "This cannot be undone.", Width: 400,
//			Scope: dlgScope, DismissID: cancelID, CloseID: closeID})
//		d.Footer(func() {
//			f.Button(cancelID, "Cancel").Outline()
//			if f.Button(deleteID, "Delete").Destructive().Clicked() { m.delete() }
//		})
//		if d.Dismissed() { m.confirm = false }
//	}
func (f *Frame) Dialog(spec DialogSpec) Box {
	layer := spec.Layer
	if layer == 0 {
		layer = f.widgets.layer + 2
	}
	width := spec.Width
	if width <= 0 {
		width = 448
	}
	// The sizes of the header and footer, and the padding, are components.Dialog's.
	geometry := components.BeginDialog(f.canvas, f.theme, f.bounds, components.DialogOptions{ContentSize: Size{Width: width}, FitBody: true}).Layout(spec.Description)
	padding := geometry.Header.X - geometry.Content.X
	gap := f.theme.Metrics.GapItem
	// The body sits as far under the header, and above the footer, as the dialog
	// leaves between them.
	above := geometry.Body.Y - (geometry.Header.Y + geometry.Header.Height)
	below := geometry.Footer.Y - (geometry.Body.Y + geometry.Body.Height)

	root := f.Popup().At(0, 0).Fill()
	r := root.n()
	r.dir = stack
	r.flags |= flagDialog
	r.aux = int32(len(f.widgets.dialogs))
	d := dialogDecl{spec: spec, layer: layer}
	root.Children(func() {
		surface := f.Column().Width(width).MaxWidth(max(f.bounds.Width-32, 1)).MaxHeight(max(f.bounds.Height-32, 1)).Self(Center).Pad(padding)
		if spec.Height > 0 {
			surface.Height(spec.Height)
		}
		surface.Children(func() {
			d.header = f.Slot().Height(geometry.Header.Height).i
			d.body = f.Column().Grow(1).Shrink(1).MarginTop(above).MarginBottom(below).i
			d.footer = f.Row().Height(geometry.Footer.Height).Gap(gap).Justify(End).Items(Center).i
		})
	})
	f.widgets.dialogs = append(f.widgets.dialogs, d)
	return root
}

// Dialog declares a dialog on a page whose targets and layer the application
// names itself: DialogSpec.Scope, DismissID, CloseID and Layer keep the
// identities the application dispatches, as they do for Frame.Dialog. Most
// dialogs use Modal or ModalWith; use Dialog when the application owns those
// targets and the dialog's layer. Header declares controls in the title row,
// Body the content and Footer the buttons.
func Dialog(c C, spec DialogSpec) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Dialog is for a page")
	}
	p := c.page
	p.forbidInMemo("Dialog")
	p.overlay = true
	return c.f.Dialog(spec)
}

// Header declares content in a Dialog's header slot, under the title when the
// Dialog has one. It lets a page put its own controls in the title row, as a
// command palette puts its filter switch there; the title remains the
// accessible name.
func (b Box) Header(fn func()) Box { return b.dialogPart(fn, dialogHeaderPart) }

// Body declares the content of a Dialog between its header and its footer.
func (b Box) Body(fn func()) Box { return b.dialogPart(fn, dialogBodyPart) }

// Footer declares the buttons of a Dialog, laid out against its right edge in the
// order given, so the last one sits in the corner.
func (b Box) Footer(fn func()) Box { return b.dialogPart(fn, dialogFooterPart) }

const (
	dialogHeaderPart = iota
	dialogBodyPart
	dialogFooterPart
)

func (b Box) dialogPart(fn func(), partKind int) Box {
	if b.n().flags&flagDialog == 0 {
		return b
	}
	d := &b.f.widgets.dialogs[b.n().aux]
	saved := b.f.widgets.layer
	b.f.widgets.layer = d.layer
	part := d.body
	switch partKind {
	case dialogHeaderPart:
		part = d.header
	case dialogFooterPart:
		part = d.footer
	}
	Box{f: b.f, i: part, gen: b.f.gen}.Children(fn)
	b.f.widgets.layer = saved
	return b
}

// Dismissed reports whether this paint is delivering a click that closes a Dialog
// or a Drawer: on its scrim (DismissID, which Escape reports too) or on its close
// button; or a Popover: a press outside it or on its anchor, or Escape.
func (b Box) Dismissed() bool {
	n := b.n()
	if n.flags&flagPopover != 0 && b.f.canvas != nil {
		return b.f.canvas.Clicked(b.f.widgets.popovers[n.aux].spec.DismissID)
	}
	if n.flags&flagDrawer != 0 && b.f.canvas != nil {
		spec := &b.f.widgets.drawers[n.aux].spec
		return b.f.canvas.Clicked(spec.dismiss) || b.f.canvas.Clicked(spec.close)
	}
	if n.flags&flagDialog == 0 || b.f.canvas == nil {
		return false
	}
	spec := &b.f.widgets.dialogs[n.aux].spec
	return b.f.canvas.Clicked(spec.DismissID) || b.f.canvas.Clicked(spec.CloseID)
}

// paintDialog paints a Dialog's root: components.Dialog's scrim and surface, then the
// body and footer inside the layer and focus scope it opens, then its modal bookkeeping.
func (f *Frame) paintDialog(n *node) {
	d := &f.widgets.dialogs[n.aux]
	surface := f.nodes[n.first].rect
	dialog := components.BeginDialog(f.canvas, f.theme, n.rect, components.DialogOptions{
		Layer: d.layer, Scope: d.spec.Scope, DismissID: d.spec.DismissID, CloseID: d.spec.CloseID,
		InitialFocusID: d.spec.InitialFocusID, ContentSize: Size{Width: surface.Width, Height: surface.Height},
	})
	if d.spec.NoClose {
		dialog = dialog.WithoutCloseButton()
	}
	if n.flags&flagDisabled != 0 {
		dialog = dialog.Disabled()
	}
	dialog.Begin(d.spec.Title, d.spec.Description)
	for c := n.first; c >= 0; c = f.nodes[c].next {
		f.paint(c)
	}
	dialog.End()
}
