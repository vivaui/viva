package base

import (
	"github.com/vivaui/viva/gfx"
)

// DialogSpec declares a Dialog. The identities are the application's, as with
// components.DialogOptions.
type DialogSpec struct {
	Title, Description string
	// Width is the surface's width, 448 when zero (kept 32 pixels inside the frame).
	// Height is its height; zero fits the body.
	Width, Height float32
	// BodyHeight is the height of a body the application paints rather than
	// declares, such as a form whose own scrolling area sizes it; zero fits the
	// body to the children declared in it. A dialog given a BodyHeight is as
	// tall as components.Dialog's FitBody layout makes it, with its body
	// exactly that tall between the header and the footer.
	BodyHeight float32
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
	// Header is the title's metrics; zero keeps the overlay dialog's.
	// components.AlertDialogHeader gives a confirmation the alert dialog's.
	Header DialogHeader
}

type DialogDecl struct {
	spec                 DialogSpec
	header, body, footer int32
	layer                uint16
}

// Dialog declares a dialog on a page whose targets and layer the application
// names itself: DialogSpec.Scope, DismissID, CloseID and Layer keep the
// identities the application dispatches, as they do for frame.Dialog. Most
// dialogs use Modal or ModalWith; use Dialog when the application owns those
// targets and the dialog's layer. Header declares controls in the title row,
// Body the content and Footer the buttons.
func Dialog(c C, spec DialogSpec) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Dialog is for a page")
	}
	p := c.page
	p.ForbidInMemo("Dialog")
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
	if b.N().Flags&flagDialog == 0 {
		return b
	}
	d := &WidgetsOf(b.f).dialogs[b.N().Aux]
	saved := WidgetsOf(b.f).layer
	WidgetsOf(b.f).layer = d.layer
	part := d.body
	switch partKind {
	case dialogHeaderPart:
		part = d.header
	case dialogFooterPart:
		part = d.footer
	}
	Box{f: b.f, i: part, gen: b.f.gen}.Children(fn)
	WidgetsOf(b.f).layer = saved
	return b
}

// Dismissed reports whether this paint is delivering a click that closes a Dialog
// or a Drawer: on its scrim (DismissID, which Escape reports too) or on its close
// button; or a Popover: a press outside it or on its anchor, or Escape.
func (b Box) Dismissed() bool {
	n := b.N()
	if n.Flags&flagPopover != 0 && b.f.Surface != nil {
		return b.f.Surface.Clicked(WidgetsOf(b.f).popovers[n.Aux].spec.DismissID)
	}
	if n.Flags&flagDrawer != 0 && b.f.Surface != nil {
		spec := WidgetsOf(b.f).drawers[n.Aux].SpecPtr()
		return b.f.Surface.Clicked(spec.dismiss) || b.f.Surface.Clicked(spec.close)
	}
	if n.Flags&flagDialog == 0 || b.f.Surface == nil {
		return false
	}
	spec := &WidgetsOf(b.f).dialogs[n.Aux].spec
	return b.f.Surface.Clicked(spec.DismissID) || b.f.Surface.Clicked(spec.CloseID)
}
