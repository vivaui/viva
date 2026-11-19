// Package inputotpview is the opt-in page control for the grouped one-time-code
// slots of package components. It is a real text field backed by a
// caller-owned text.Editor:
//
//	code := inputotpview.New().
//		Bind(&m.codeField, m.codeEditor).
//		Slots(6).GroupSize(3).Filter(inputotpview.Digits)
//	code.View(c, "Verification code")
//	if code.Clicked() {
//		// a press on the control, already moved to the slot under the pointer
//	}
//
// The application owns the field and its editor; the view paints the editor's
// value, caret, selection and composition into the slots and gives the
// platform text service the same editor. Slots, GroupSize, Filter, Invalid,
// Disabled, ReadOnly and the static snapshot options are chained; there is no
// second constructor. The filter restricts typing and paste to the code's
// characters and overwrite mode fills the slot under the caret, as a fixed-width
// code should.
//
// View declares a ui.Control with a text-field role. By default the field's
// hit is the control's own identity, so a page configured with ui.Configure
// gives its platform the editor through the control's Text handler. FieldID
// names the hit explicitly for an embedding runner that resolves text by an
// application identity; SelectAt lets such a runner route a press to a slot.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an OTP in the model beside its editor: its handlers and
// draw function are made once, so a warm paint allocates nothing. Do not copy
// an OTP after it has been declared.
package inputotpview

import (
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Filter selects which characters the code accepts. It is
// components.InputOTPFilter, named here so a page reads through this package
// alone.
type Filter = components.InputOTPFilter

const (
	// Any accepts every visible character.
	Any Filter = components.InputOTPAny
	// Digits accepts the ASCII digits 0-9, the default.
	Digits Filter = components.InputOTPDigits
	// Alphanumeric accepts the ASCII digits and letters.
	Alphanumeric Filter = components.InputOTPAlphanumeric
)

// OTP is one one-time-code field and its page control.
type OTP struct {
	field   *ui.Field
	editor  *text.Editor
	fieldID gfx.ID

	value  string
	slots  int
	group  int
	filter Filter

	invalid  bool
	disabled bool
	readOnly bool

	focused      bool
	showCaret    bool
	activeSlot   int
	hasSelection bool
	selStart     int
	selEnd       int
	hasCompose   bool
	compStart    int
	compEnd      int

	label     string
	paintedID gfx.ID
	layout    components.InputOTPLayout
	input     window.TextInput

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	clicked  gfx.ID
	hadClick bool
}

// New returns an empty six-slot code. Give it a field or a static Value.
func New() *OTP { return &OTP{slots: 6, filter: Digits} }

// Bind gives the view the caller-owned field and the editor its slots paint and
// the platform text service edits. Either may be nil: an editor alone paints
// the code without a page field, and a field alone paints its text without
// slot editing. Bind applies the filter and overwrite mode to the editor.
func (o *OTP) Bind(field *ui.Field, editor *text.Editor) *OTP {
	o.field, o.editor = field, editor
	o.restrict()
	return o
}

// Slots sets how many characters the code has. The default is six.
func (o *OTP) Slots(count int) *OTP {
	if count <= 0 {
		count = 6
	}
	o.slots = count
	o.restrict()
	return o
}

// GroupSize sets how many slots share one rounded border before a separator.
// Zero or a value not smaller than Slots paints one group.
func (o *OTP) GroupSize(size int) *OTP {
	o.group = size
	return o
}

// Filter selects which characters may be typed or pasted. The default is
// Digits.
func (o *OTP) Filter(filter Filter) *OTP {
	o.filter = filter
	o.restrict()
	return o
}

// Value is the code painted when there is no editor.
func (o *OTP) Value(value string) *OTP {
	o.value = value
	return o
}

// Invalid paints the error state.
func (o *OTP) Invalid() *OTP {
	o.invalid = true
	return o
}

// Disabled paints the code inert.
func (o *OTP) Disabled() *OTP {
	o.disabled = true
	return o
}

// ReadOnly paints the code focused but not editable.
func (o *OTP) ReadOnly() *OTP {
	o.readOnly = true
	return o
}

// Editable clears ReadOnly, for a page that enables editing after the first
// paint.
func (o *OTP) Editable() *OTP {
	o.readOnly = false
	return o
}

// Focused shows the focus ring and active slot for a static snapshot.
func (o *OTP) Focused() *OTP {
	o.focused = true
	return o
}

// ShowCaret shows the caret in the active empty slot of a static snapshot. An
// editor shows it whenever the slot is focused and the selection is collapsed.
func (o *OTP) ShowCaret() *OTP {
	o.showCaret = true
	return o
}

// ActiveSlot selects the active slot of a static snapshot.
func (o *OTP) ActiveSlot(index int) *OTP {
	o.activeSlot = index
	return o
}

// Select marks the selected slot range of a static snapshot.
func (o *OTP) Select(start, end int) *OTP {
	o.hasSelection, o.selStart, o.selEnd = start != end, start, end
	return o
}

// Composition marks the input-method composition range of a static snapshot.
func (o *OTP) Composition(start, end int) *OTP {
	o.hasCompose, o.compStart, o.compEnd = start != end, start, end
	return o
}

// FieldID gives the field's hit an explicit identity instead of the control's
// own, for a runner that resolves the platform text service by an application
// identity.
func (o *OTP) FieldID(id gfx.ID) *OTP {
	o.fieldID = id
	return o
}

// View declares the code at c. label names the field for assistive technology.
// The box is the slots' intrinsic size; give it a width to place it.
//
// Declare the view on its own, not inside a Memo.
func (o *OTP) View(c ui.C, label string) ui.Box {
	if o == nil {
		panic("inputotpview: View needs its OTP")
	}
	o.prepare()
	o.label = label
	layout := components.PaintInputOTP(nil, *c.Theme(), gfx.Rect{}, components.InputOTPSnapshot{
		Slots: o.slots, GroupSize: o.group,
	})
	return ui.Control(c, label, &o.handlers).Size(layout.Bounds.Width, layout.Bounds.Height).Draw(o.drawFn)
}

// Layout is the slot geometry painted last. It is zero until View has painted.
func (o *OTP) Layout() components.InputOTPLayout { return o.layout }

// Clicked reports whether the paint just made delivered a press on the code.
// It is false in ordinary paints.
func (o *OTP) Clicked() bool {
	return o.hadClick && !o.paintedID.IsZero() && o.clicked == o.paintedID
}

// SelectAt moves the caret to the slot under x, in window coordinates, or to
// the end when the press is past the last slot. It reports whether it acted.
// It is for a runner that paints the field under an external identity and routes
// the press itself; the control's own pointer handler calls the same method.
func (o *OTP) SelectAt(x float32) bool {
	if o.editor == nil || o.disabled || o.readOnly || o.layout.Slots == 0 {
		return false
	}
	slot := o.layout.Slots - 1
	for index := 0; index < o.layout.Slots; index++ {
		bounds := o.layout.SlotBounds(index)
		if x < bounds.X+bounds.Width {
			slot = index
			break
		}
	}
	caret := min(slot, o.editor.RuneCount())
	o.editor.SetSelection(text.Range{Start: caret, End: caret})
	return true
}

func (o *OTP) restrict() {
	if o.editor == nil {
		return
	}
	o.editor.Restrict(o.filter.Accepts, o.slotCount())
	o.editor.SetOverwrite(true)
}

func (o *OTP) slotCount() int {
	if o.slots <= 0 {
		return 6
	}
	return o.slots
}

func (o *OTP) prepare() {
	if o.prepared {
		return
	}
	o.prepared = true
	o.handlers.Role = gfx.RoleGroup
	o.handlers.Key = o.key
	o.handlers.Pointer = o.pointer
	o.handlers.Text = o.text
	o.drawFn = o.draw
}

func (o *OTP) text(window.TextInputContext) *window.TextInput {
	if o.editor == nil {
		return nil
	}
	o.input = window.TextInput{Editor: o.editor}
	return &o.input
}

func (o *OTP) key(event gfx.KeyEvent) gfx.EventResult {
	if o.editor == nil || o.disabled || o.readOnly {
		return gfx.EventResult{}
	}
	if !o.paintedID.IsZero() && event.Target != o.paintedID {
		return gfx.EventResult{}
	}
	command, ok := components.InputOTPCommandForKey(components.InputOTPSnapshot{
		Disabled: o.disabled, ReadOnly: o.readOnly,
	}, event.Key)
	if !ok {
		return gfx.EventResult{}
	}
	extend := event.Modifiers&gfx.ModifierShift != 0
	switch command {
	case components.InputOTPCommandMovePrevious:
		o.editor.MoveLeft(extend)
	case components.InputOTPCommandMoveNext:
		o.editor.MoveRight(extend)
	case components.InputOTPCommandMoveFirst:
		o.editor.MoveTo(0, extend)
	case components.InputOTPCommandMoveLast:
		o.editor.MoveTo(o.editor.RuneCount(), extend)
	case components.InputOTPCommandDeleteBackward:
		o.editor.DeleteBackward()
	case components.InputOTPCommandDeleteForward:
		o.editor.DeleteForward()
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (o *OTP) pointer(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	if !o.paintedID.IsZero() && event.Target != o.paintedID {
		return gfx.EventResult{}
	}
	if !o.SelectAt(event.X) {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (o *OTP) draw(d *ui.DrawContext) {
	id := o.fieldID
	if id.IsZero() {
		id = d.Node.ID
	}
	o.paintedID = id
	o.clicked, o.hadClick = d.Canvas.ClickTarget()

	slots := o.slotCount()
	focused := o.focused || (!id.IsZero() && d.Canvas.Focused() == id)
	value := o.value
	caret := o.activeSlot
	selStart, selEnd := o.selStart, o.selEnd
	hasSelection := o.hasSelection
	compStart, compEnd := o.compStart, o.compEnd
	hasCompose := o.hasCompose
	showCaret := o.showCaret
	if o.editor != nil {
		value = o.editor.String()
		selection := o.editor.Selection()
		caret = o.editor.Caret()
		selStart, selEnd = selection.Start, selection.End
		hasSelection = selStart != selEnd
		hasCompose = false
		if composition, composing := o.editor.Composition(); composing {
			hasCompose, compStart, compEnd = true, composition.Start, composition.End
		}
		showCaret = focused && !hasSelection && !o.disabled
	} else if o.field != nil {
		input := o.field.Snapshot(d.Frame, id, o.label, "")
		value = input.Value
		caret = utf8.RuneCountInString(input.Value[:clampOffset(input.Value, input.Caret)])
		selStart = utf8.RuneCountInString(input.Value[:clampOffset(input.Value, input.SelectionStart)])
		selEnd = utf8.RuneCountInString(input.Value[:clampOffset(input.Value, input.SelectionEnd)])
		compStart = utf8.RuneCountInString(input.Value[:clampOffset(input.Value, input.CompositionStart)])
		compEnd = utf8.RuneCountInString(input.Value[:clampOffset(input.Value, input.CompositionEnd)])
		hasSelection = selStart != selEnd
		hasCompose = compStart != compEnd
		focused = focused || input.Focused
		showCaret = showCaret || input.ShowCaret
	}
	active := min(caret, slots-1)
	snapshot := components.InputOTPSnapshot{
		ID: id, Label: o.label, Value: value, Slots: slots, GroupSize: o.group,
		Disabled: o.disabled, ReadOnly: o.readOnly, Invalid: o.invalid,
		Focused: focused, ShowCaret: !o.disabled && showCaret,
		PlatformInput: o.editor != nil && !o.disabled && !o.readOnly,
		HasActiveSlot: focused && !o.disabled, ActiveSlot: active,
		HasSelection: hasSelection, SelectionStart: selStart, SelectionEnd: selEnd,
		HasComposition: hasCompose, CompositionStart: compStart, CompositionEnd: compEnd,
	}
	o.layout = components.PaintInputOTP(d.Canvas, *d.Theme, d.Rect, snapshot)
}

func clampOffset(value string, offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(value) {
		return len(value)
	}
	return offset
}
