package flex

import (
	"unsafe"

	"github.com/vivaui/viva/ui"
)

// Popover shows a panel body declares, hanging off anchor, while *open is true: a
// press outside it or Escape sets *open to false (and the press goes on to what is
// under the pointer), and a press on the anchor only closes it, so the anchor that
// toggles it does not open it again. The focus goes back to where it was as it
// closes. It reports the paint that closed it, so a panel that must stay open sets
// *open back.
//
//	more := s.Button("More").Outline()
//	if more.Clicked() { m.menu = !m.menu }
//	s.Popover(more, &m.menu, func(p flex.Screen) {
//		p.Checkbox(&m.online, "Online only")
//	})
//
// open names the panel: give each popover its own.
func (s Screen) Popover(anchor Box, open *bool, body func(Screen)) bool {
	return s.PopoverWith(anchor, PopoverSpec{}, open, body)
}

// PopoverWith is Popover with a spec: the side, alignment, width and the rest of
// PopoverSpec. The Scope and DismissID it leaves zero are made from open.
func (s Screen) PopoverWith(anchor Box, spec PopoverSpec, open *bool, body func(Screen)) bool {
	f := s.frame()
	if !*open {
		return false
	}
	scope := s.scopeOf(skPopover, unsafe.Pointer(open))
	if spec.Scope.IsZero() {
		spec.Scope = scope.At(0)
	}
	if spec.DismissID.IsZero() {
		spec.DismissID = scope.At(1)
	}
	panel := f.Popover(anchor, spec)
	inner := s.child(scope, false)
	panel.Children(func() { body(inner) })
	if panel.Dismissed() {
		*open = false
		return true
	}
	return false
}

// ModalSpec declares a Modal: the dialog's title and description, its size (448
// wide and as tall as its body when zero), and NoClose to leave out the close button.
// InitialFocus, when set, is the control that takes the keyboard as the dialog opens;
// otherwise the first control in the body does, as shadcn's Dialog.
type ModalSpec struct {
	Title, Description string
	Width, Height      float32
	NoClose            bool
	InitialFocus       ui.ID
}

// ModalScreen is the Screen of a Modal's body, with the dialog's footer.
type ModalScreen struct {
	Screen
	dialog Box
}

// Footer declares the dialog's buttons, laid out against its right edge in the order
// given, so the last one sits in the corner.
func (m ModalScreen) Footer(fn func()) { m.dialog.Footer(fn) }

// Modal shows a dialog titled title over the dimmed page while *open is true, with
// body declaring what is in it: Escape, a press on the dim and the close button set
// *open to false. It keeps the keyboard while it is open, gives it to the first
// control in the body as it opens, and back to where it was as it closes. It reports
// the paint that closed it. The title is the dialog's name for accessibility, so it
// is required; open names the dialog.
//
//	if s.Button("Delete…").Destructive().Clicked() { m.confirm = true }
//	s.Modal(&m.confirm, "Delete host?", func(d flex.ModalScreen) {
//		d.Text("This cannot be undone.").Muted()
//		d.Footer(func() {
//			if d.Button("Cancel").Outline().Clicked() { m.confirm = false }
//			if d.Button("Delete").Destructive().Clicked() { m.delete(); m.confirm = false }
//		})
//	})
func (s Screen) Modal(open *bool, title string, body func(ModalScreen)) bool {
	return s.ModalWith(ModalSpec{Title: title}, open, body)
}

// ModalWith is Modal with a description, a size or no close button.
func (s Screen) ModalWith(spec ModalSpec, open *bool, body func(ModalScreen)) bool {
	f := s.frame()
	if !*open {
		return false
	}
	if spec.Title == "" {
		panic("flex: Screen.Modal needs a title: it is the dialog's name for assistive technology")
	}
	scope := s.scopeOf(skModal, unsafe.Pointer(open))
	dialog := f.Dialog(DialogSpec{
		Title: spec.Title, Description: spec.Description, Width: spec.Width, Height: spec.Height,
		Scope: scope.At(0), DismissID: scope.At(1), CloseID: scope.At(2), InitialFocusID: spec.InitialFocus,
		NoClose: spec.NoClose,
	})
	inner := ModalScreen{Screen: s.child(scope, false), dialog: dialog}
	dialog.Body(func() { body(inner) })
	if spec.InitialFocus.IsZero() {
		f.focusFirstControl(dialog)
	}
	if dialog.Dismissed() {
		*open = false
		return true
	}
	return false
}

// focusFirstControl makes the first control declared in a Dialog's body, or its
// footer, the one that takes the keyboard as it opens; the close button when it has
// none.
func (f *Frame) focusFirstControl(dialog Box) {
	d := &f.widgets.dialogs[dialog.n().aux]
	for _, part := range [2]int32{d.body, d.footer} {
		if id := f.firstControl(part); !id.IsZero() {
			d.spec.InitialFocusID = id
			return
		}
	}
	d.spec.InitialFocusID = d.spec.CloseID
}

// firstControl is the first element under i, in declaration order, with an identity
// the keyboard can take: a control, or a container given Hit.
func (f *Frame) firstControl(i int32) ui.ID {
	for c := f.nodes[i].first; c >= 0; c = f.nodes[c].next {
		n := &f.nodes[c]
		if n.flags&flagDisabled == 0 && !n.id.IsZero() {
			switch n.kind {
			case kindButton, kindCheckbox, kindSwitch, kindInput, kindTextarea, kindSlider:
				return n.id
			case kindContainer:
				if n.flags&flagHasHit != 0 {
					return n.id
				}
			}
		}
		if id := f.firstControl(c); !id.IsZero() {
			return id
		}
	}
	return ui.ID{}
}
