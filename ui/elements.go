package ui

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/gfx"
)

// Containers. Each has a name in its parent, so what is declared inside it is
// named within it.

func Row(c C) Box    { return c.page.container(c.f.Row()) }
func Column(c C) Box { return c.page.container(c.f.Column()) }
func Stack(c C) Box  { return c.page.container(c.f.Stack()) }

// Scroll is a scroll area whose offset is kept by the area's own name.
func Scroll(c C) Box {
	p := c.page
	p.forbidInMemo("Scroll")
	at, _ := p.name(pkContainer)
	b := c.f.ScrollArea(p.id(at))
	p.setNode(b.i, at)
	return b
}

// Leaves without identity.

func Text(c C, value string) Box { return c.f.Text(value) }

// Textf allocates its string in every paint.
func Textf(c C, format string, args ...any) Box { return c.f.Text(fmt.Sprintf(format, args...)) }
func Spacer(c C) Box                            { return c.f.Spacer() }
func Divider(c C) Box                           { return c.f.Divider() }
func Progress(c C, fraction float32) Box        { return c.f.Progress(fraction) }
func Spinner(c C) Box                           { return c.f.Spinner() }
func Icon(c C, icon gfx.Icon) Box               { return c.f.Icon(icon) }
func Image(c C, bitmap gfx.Bitmap) Box          { return c.f.Image(bitmap) }

// Paragraph is text that wraps to the width it gets, at most maxLines lines.
func Paragraph(c C, value string, maxLines int) Box { return c.f.Paragraph(value, maxLines) }

// Slot reserves room for something painted by hand (Draw) or a native view
// (NativeView): Rect says where, once the page is laid out.
func Slot(c C) Box { return c.f.Slot() }

// Grid lays its children in equal cells at least minCell wide.
func Grid(c C, minCell float32) Box { return c.page.container(c.f.Grid(minCell)) }

// Controls.

// Button adds a button; Clicked reports a click on it.
func Button(c C, label string) Box {
	id := c.page.controlID(pkButton)
	b := c.f.Button(id, label)
	c.page.guard(b, id, label)
	return b
}

// Checkbox adds a checkbox showing *value, flipped by a click, with label beside it.
func Checkbox(c C, value *bool, label string) Box {
	b := c.f.Checkbox(c.page.boundID(pkCheckbox, unsafe.Pointer(value)), *value)
	if label != "" {
		b.Caption(label)
	}
	c.f.bindBool(b, value)
	return b
}

// Switch is Checkbox's two-state switch.
func Switch(c C, value *bool, label string) Box {
	b := c.f.Switch(c.page.boundID(pkSwitch, unsafe.Pointer(value)), *value)
	if label != "" {
		b.Caption(label)
	}
	c.f.bindBool(b, value)
	return b
}

// optionLabel is the label of an option: label's, or the option itself when
// label is nil and the options are strings.
func optionLabel[T comparable](option T, label func(T) string) string {
	if label != nil {
		return label(option)
	}
	if text, ok := any(option).(string); ok {
		return text
	}
	panic("ui: options that are not strings need a label function")
}

// options writes the labels of options into the page's scratch, and returns them
// and the index of *value among the options (-1 when it is none of them).
func options[T comparable](p *Page, value *T, options []T, label func(T) string) ([]string, int) {
	first := len(p.labels)
	selected := -1
	for i, option := range options {
		p.labels = append(p.labels, optionLabel(option, label))
		if option == *value {
			selected = i
		}
	}
	return p.labels[first:len(p.labels):len(p.labels)], selected
}

// pick writes the option a click chose into *value, where the control is declared,
// and records whether it did for Changed.
func pick[T comparable](c C, b Box, value *T, options []T, index int, ok bool) {
	changed := ok && index < len(options) && options[index] != *value
	if changed {
		*value = options[index]
	}
	c.f.bindDone(b, changed)
}

// Choice is a segmented control over options, *value the chosen one; a click sets
// it. The options are of the value's type, so a choice of the wrong kind does not
// compile; label names them, or is nil for options that are strings.
//
//	ui.Choice(c, &m.theme, []Theme{Light, Dark}, Theme.String)
//	ui.Choice(c, &m.size, []string{"S", "M", "L"}, nil)
func Choice[T comparable](c C, value *T, choices []T, label func(T) string) Box {
	p := c.page
	scope := p.boundScope(pkChoice, unsafe.Pointer(value))
	labels, selected := options(p, value, choices, label)
	b := c.f.Choice(scope, labels, selected)
	p.rove(c.f, b, false)
	index, ok := b.ChosenIndex()
	pick(c, b, value, choices, index, ok)
	return b
}

// Tabs is a tab strip over tabs, *value the active one; a click sets it. Declare
// what the active tab shows after it.
func Tabs[T comparable](c C, value *T, tabs []T, label func(T) string) Box {
	p := c.page
	scope := p.boundScope(pkTabs, unsafe.Pointer(value))
	labels, selected := options(p, value, tabs, label)
	b := c.f.Tabs(scope, labels, selected)
	p.rove(c.f, b, true)
	index, ok := b.TabClicked()
	pick(c, b, value, tabs, index, ok)
	return b
}

// Select is a drop-down over choices, *value the chosen one (Placeholder shows while
// it is none of them); picking an option sets it. Whether its list is open is the
// Select's own state.
func Select[T comparable](c C, value *T, choices []T, label func(T) string) Box {
	return selectOf(c, value, choices, label, false)
}

// selectOf is Select, or SelectBase when base is set: the same control without the theme's look.
func selectOf[T comparable](c C, value *T, choices []T, label func(T) string, base bool) Box {
	p := c.page
	scope := p.boundScope(pkSelect, unsafe.Pointer(value))
	state := stateIn[SelectState](p, scope, "select") // its open list follows its value
	labels, selected := options(p, value, choices, label)
	var b Box
	if base {
		b = c.f.SelectBase(scope, state, labels, selected)
	} else {
		b = c.f.Select(scope, state, labels, selected)
	}
	p.selects = append(p.selects, state)
	index, ok := b.Picked()
	pick(c, b, value, choices, index, ok)
	if state.Open {
		p.overlay = true // its open list holds the pointer and keys as a Popover does
	}
	return b
}

// Slider is a slider over state.Value; Range, Step and Changed are the Frame's.
func Slider(c C, state *SliderState) Box {
	c.page.slides(state)
	return c.f.Slider(c.page.boundID(pkSlider, unsafe.Pointer(state)), state)
}

// Splitter is a draggable divider between the panes around it, at state.Pos.
func Splitter(c C, state *SplitterState) Box {
	return c.f.Splitter(c.page.boundID(pkSplitter, unsafe.Pointer(state)), state)
}

// Table is a Table; its Rows and Header scopes are made from state when spec leaves
// them zero.
func Table(c C, state *TableState, spec TableSpec) Box {
	p := c.page
	if spec.Rows == 0 || spec.Header == 0 {
		scope := p.boundScope(pkTable, unsafe.Pointer(state))
		if spec.Rows == 0 {
			spec.Rows = p.tree.Sub(scope, gfx.Index(0))
		}
		if spec.Header == 0 {
			spec.Header = p.tree.Sub(scope, gfx.Index(1))
		}
	}
	return c.f.Table(state, spec)
}

// TextField adds a text field editing value, a string of the model or a Field; the
// page gives the platform its editor. Changed is true in the paint that finds it
// edited, and the string already holds the edit then.
//
//	ui.TextField(c, &m.name, "Name", "Your name")
func TextField[T TextValue](c C, value T, label, placeholder string) Box {
	return textFieldOf(c, value, label, placeholder, false)
}

// textFieldOf is TextField, or TextFieldBase when base is set.
func textFieldOf[T TextValue](c C, value T, label, placeholder string, base bool) Box {
	p := c.page
	field, id := fieldOf(p, pkField, value)
	var b Box
	if base {
		b = c.f.FieldBase(field, id, label, placeholder)
	} else {
		b = c.f.Field(field, id, label, placeholder)
	}
	c.f.bindField(b, field)
	p.fields = append(p.fields, fieldEntry{id: id, field: field})
	return b
}

// TextArea adds a text field of several rows editing field: Enter starts a new
// line, long lines wrap and the rows scroll to follow the caret. Give it a Height.
// The page gives the platform its editor, as for a TextField; it has no Submitted.
//
//	ui.TextArea(c, &m.notes, "Notes", "Anything to add").Height(120)
func TextArea[T TextValue](c C, value T, label, placeholder string) Box {
	p := c.page
	field, id := fieldOf(p, pkArea, value)
	b := c.f.FieldArea(field, id, label, placeholder)
	c.f.bindField(b, field)
	if p.areaKey == nil {
		p.keyRoute, p.areaKey = (*Page).routeKey, (*Field).Key // Up and Down by row
	}
	p.fields = append(p.fields, fieldEntry{id: id, field: field})
	return b
}

// Popover shows the panel body declares, hanging off anchor, while *open: a press
// outside it or Escape sets *open to false (and the press goes on to what is under
// the pointer), and a press on the anchor only closes it, so the anchor that toggles
// it does not open it again. The focus goes back to where it was as it closes. It
// reports the paint that closed it, so a panel that must stay open sets *open back.
//
//	more := ui.Button(c, "More")
//	if more.Clicked() { m.menu = !m.menu }
//	ui.Popover(c, more, &m.menu, func() { ... })
func Popover(c C, anchor Box, open *bool, body func()) bool {
	return PopoverWith(c, anchor, PopoverSpec{Align: Start}, open, body)
}

// PopoverWith is Popover with a spec: the side, alignment, width and the rest of
// PopoverSpec. The Scope and DismissID it leaves zero are made from open.
func PopoverWith(c C, anchor Box, spec PopoverSpec, open *bool, body func()) bool {
	p := c.page
	p.forbidInMemo("Popover")
	if !*open {
		return false
	}
	p.overlay = true
	scope := p.boundScope(pkPopover, unsafe.Pointer(open))
	if spec.Scope.IsZero() {
		spec.Scope = scope.At(0)
	}
	if spec.DismissID.IsZero() {
		spec.DismissID = scope.At(1)
	}
	panel := c.f.Popover(anchor, spec)
	p.setScope(panel.i, scope)
	panel.Children(body)
	if panel.Dismissed() {
		*open = false
		return true
	}
	return false
}

// ModalSpec declares a Modal beyond its title: a description, its size (448 wide
// and as tall as its body when zero), NoClose to leave out the close button, and
// InitialFocus, the control that takes the keyboard as it opens (the first control
// of the body when zero, as shadcn's Dialog).
type ModalSpec struct {
	Description   string
	Width, Height float32
	NoClose       bool
	InitialFocus  gfx.ID
}

// Modal shows a dialog titled title over the dimmed window while *open, with body
// declaring what is in it and footer, when not nil, its buttons (against the right
// edge in the order given). Escape, a press on the dim and the close button set
// *open to false. It keeps the keyboard while open, gives it to the first control as
// it opens and back to where it was as it closes, and reports the paint that closed
// it. The title is the dialog's name for assistive technology. body and footer may
// be nil.
func Modal(c C, open *bool, title string, body, footer func()) bool {
	return ModalWith(c, open, title, ModalSpec{}, body, footer)
}

// ModalWith is Modal with a description, a size, no close button or an initial focus.
func ModalWith(c C, open *bool, title string, spec ModalSpec, body, footer func()) bool {
	p := c.page
	p.forbidInMemo("Modal")
	if !*open {
		return false
	}
	p.overlay = true
	if title == "" {
		panic("ui: Modal needs a title: it is the dialog's name for assistive technology")
	}
	scope := p.boundScope(pkModal, unsafe.Pointer(open))
	dialog := c.f.Dialog(DialogSpec{
		Title: title, Description: spec.Description, Width: spec.Width, Height: spec.Height, NoClose: spec.NoClose,
		Scope: scope.At(0), DismissID: scope.At(1), CloseID: scope.At(2), InitialFocusID: spec.InitialFocus,
	})
	d := &c.f.widgets.dialogs[dialog.n().aux]
	p.setScope(d.body, scope)
	p.setScope(d.footer, scope)
	if body != nil {
		dialog.Body(body)
	}
	if footer != nil {
		dialog.Footer(footer)
	}
	if spec.InitialFocus.IsZero() {
		c.f.focusFirstControl(dialog)
	}
	if dialog.Dismissed() {
		*open = false
		return true
	}
	return false
}

// Describe writes an identity of the page as its path, for test failures.
func (p *Page) Describe(id gfx.ID) string { return p.tree.Text(id) }

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
func (f *Frame) firstControl(i int32) gfx.ID {
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
	return gfx.ID{}
}
