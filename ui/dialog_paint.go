package ui

import (
	"github.com/vivaui/viva/components"
)

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
func (f *frame) Dialog(spec DialogSpec) Box {
	layer := spec.Layer
	if layer == 0 {
		layer = f.WidgetLayer() + 2
	}
	width := spec.Width
	if width <= 0 {
		width = 448
	}
	// The sizes of the header and footer, and the padding, are components.Dialog's.
	geometry := components.BeginDialog(f.Surface, f.Palette, f.bounds, components.DialogOptions{ContentSize: Size{Width: width}, FitBody: true, BodyHeight: spec.BodyHeight, Header: spec.Header}).Layout(spec.Description)
	padding := geometry.Header.X - geometry.Content.X
	gap := f.Palette.Metrics.GapItem
	// The body sits as far under the header, and above the footer, as the dialog
	// leaves between them.
	above := geometry.Body.Y - (geometry.Header.Y + geometry.Header.Height)
	below := geometry.Footer.Y - (geometry.Body.Y + geometry.Body.Height)

	root := f.Popup().At(0, 0).Fill()
	r := root.n()
	r.Dir = stack
	r.Flags |= flagDialog
	r.Aux = int32(len(widgetsOf(f).dialogs))
	d := dialogDecl{spec: spec, layer: layer}
	root.Children(func() {
		surface := f.Column().Width(width).MaxWidth(max(f.bounds.Width-32, 1)).MaxHeight(max(f.bounds.Height-32, 1)).Self(Center).Pad(padding)
		switch {
		case spec.Height > 0:
			surface.Height(spec.Height)
		case spec.BodyHeight > 0:
			surface.Height(geometry.Content.Height)
		}
		surface.Children(func() {
			d.header = f.Slot().Height(geometry.Header.Height).i
			d.body = f.Column().Grow(1).Shrink(1).MarginTop(above).MarginBottom(below).i
			d.footer = f.Row().Height(geometry.Footer.Height).Gap(gap).Justify(End).Items(Center).i
		})
	})
	widgetsOf(f).dialogs = append(widgetsOf(f).dialogs, d)
	return root
}

// paintDialog paints a Dialog's root: components.Dialog's scrim and surface, then the
// body and footer inside the layer and focus scope it opens, then its modal bookkeeping.
func (f *frame) paintDialog(n *node) {
	d := &widgetsOf(f).dialogs[n.Aux]
	surface := f.Nodes[n.First].Rect
	dialog := components.BeginDialog(f.Surface, f.Palette, n.Rect, components.DialogOptions{
		Layer: d.layer, Scope: d.spec.Scope, DismissID: d.spec.DismissID, CloseID: d.spec.CloseID,
		InitialFocusID: d.spec.InitialFocusID, ContentSize: Size{Width: surface.Width, Height: surface.Height},
		Header: d.spec.Header,
	})
	if d.spec.NoClose {
		dialog = dialog.WithoutCloseButton()
	}
	if n.Flags&flagDisabled != 0 {
		dialog = dialog.Disabled()
	}
	dialog.Begin(d.spec.Title, d.spec.Description)
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		f.paint(c)
	}
	dialog.End()
}
